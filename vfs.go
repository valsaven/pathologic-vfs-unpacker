package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// vfsHeaderSize = Magic (4) + Version (4) + FileCount (4)
	vfsHeaderSize = 12
	// entryFixedMetadataSuffixSize = FileSize (4) + FileOffset (4) + Unknown/Padding (8)
	// Size of the fixed metadata part *after* the filename.
	// Based on the original seek logic (pos_before_size_offset + 16),
	// where pos_before_size_offset is the position *before* reading fileSize (4 bytes) and fileOffset (4 bytes),
	// the total offset was 16 bytes. This means that after fileOffset (4 bytes), there are another 16 - 4 - 4 = 8 bytes
	// before the start of the next entry (name length).
	entryFixedMetadataSuffixSize = 16
)

// vfsMagicBytes - Expected magic bytes for Pathologic VFS files.
var vfsMagicBytes = []byte("LP1C")

// supportedVFSVersion - The VFS format version supported by this unpacker.
var supportedVFSVersion = []byte{0, 0, 0, 0}

// VFSHeader represents the header of a VFS archive.
type VFSHeader struct {
	Magic     [4]byte
	Version   [4]byte
	FileCount uint32
}

// VFSEntryMetadata represents the metadata for a single file within the VFS archive.
type VFSEntryMetadata struct {
	Name       string
	FileSize   uint32
	FileOffset uint32
	// Fields below are not stored in the struct but are used for offset calculations
	// nameLength byte
	// entryStartOffset int64 // Start position of this entry's metadata
}

// Unpacker encapsulates the state and logic for unpacking a VFS file.
type Unpacker struct {
	vfsFile   *os.File
	vfsSize   int64
	outputDir string
	header    VFSHeader
	entries   []VFSEntryMetadata // Might be useful for the future, but not fully populated in advance for now
}

// NewUnpacker creates a new Unpacker instance.
func NewUnpacker(vfsPath string, outputDir string) (*Unpacker, error) {
	f, err := os.Open(vfsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open VFS file '%s': %w", vfsPath, err)
	}

	vfsInfo, err := f.Stat()
	if err != nil {
		f.Close() // Close the file on error
		return nil, fmt.Errorf("failed to get VFS file info: %w", err)
	}
	vfsSize := vfsInfo.Size()

	// Basic size check
	if vfsSize < vfsHeaderSize {
		f.Close()
		return nil, fmt.Errorf("invalid VFS file: size (%d bytes) is too small (minimum %d)", vfsSize, vfsHeaderSize)
	}

	return &Unpacker{
		vfsFile:   f,
		vfsSize:   vfsSize,
		outputDir: outputDir,
	}, nil
}

// Close closes the VFS file.
func (u *Unpacker) Close() error {
	if u.vfsFile != nil {
		return u.vfsFile.Close()
	}
	return nil
}

// readAndVerifyHeader reads and verifies the VFS file header.
func (u *Unpacker) readAndVerifyHeader() error {
	// --- 1. Read and Verify Magic Bytes ---
	magic := make([]byte, len(vfsMagicBytes))
	if _, err := io.ReadFull(u.vfsFile, magic); err != nil {
		return fmt.Errorf("failed to read magic bytes: %w", err)
	}
	if !bytes.Equal(magic, vfsMagicBytes) {
		return fmt.Errorf("invalid magic bytes: got '%s', expected '%s'. Is this a Pathologic VFS file?", string(magic), string(vfsMagicBytes))
	}
	copy(u.header.Magic[:], magic) // Store in struct

	// --- 1.1 Read and Check Version ---
	versionBytes := make([]byte, len(supportedVFSVersion))
	if _, err := io.ReadFull(u.vfsFile, versionBytes); err != nil {
		// Use Seek to get the current position for the error message
		currentPos, _ := u.vfsFile.Seek(0, io.SeekCurrent) // Ignore Seek error, as the main error is ReadFull
		return fmt.Errorf("failed to read version bytes (offset %d): %w", currentPos-int64(len(versionBytes)), err)
	}
	if !bytes.Equal(versionBytes, supportedVFSVersion) {
		return fmt.Errorf("unsupported VFS format version: got %v, expected %v", versionBytes, supportedVFSVersion)
	}
	copy(u.header.Version[:], versionBytes) // Store in struct
	fmt.Printf("Detected VFS format version: %v (Supported)\n", versionBytes)

	// --- 2. Read File Count ---
	if err := binary.Read(u.vfsFile, binary.LittleEndian, &u.header.FileCount); err != nil {
		currentPos, _ := u.vfsFile.Seek(0, io.SeekCurrent)
		return fmt.Errorf("failed to read file count (offset %d): %w", currentPos-4, err) // 4 is the size of uint32
	}

	fmt.Printf("Archive contains %d files.\n", u.header.FileCount)
	if u.header.FileCount == 0 {
		fmt.Println("No files to extract.")
		return nil // Not an error, just nothing to do
	}

	return nil
}

// createOutputDirectory creates the base output directory.
func (u *Unpacker) createOutputDirectory() error {
	fmt.Printf("Creating output directory: %s\n", u.outputDir)
	if err := os.MkdirAll(u.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create base output directory '%s': %w", u.outputDir, err)
	}
	return nil
}

// extractFiles processes all file entries and extracts them.
func (u *Unpacker) extractFiles() error {
	if u.header.FileCount == 0 {
		return nil // Nothing to extract
	}

	// Metadata starts immediately after the header (12 bytes)
	currentOffset, err := u.vfsFile.Seek(vfsHeaderSize, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to the start of file entries (offset %d): %w", vfsHeaderSize, err)
	}
	fmt.Printf("Reading file entries starting at offset %d (0x%X)\n", currentOffset, currentOffset)

	for i := uint32(0); i < u.header.FileCount; i++ {
		entryStartOffset := currentOffset // Store the start of the current entry

		// --- Read Entry Metadata ---
		var nameLength uint8
		if err := binary.Read(u.vfsFile, binary.LittleEndian, &nameLength); err != nil {
			// Check for EOF at the end of the expected file list
			if err == io.EOF && i == u.header.FileCount {
				break // Normal termination
			}
			return fmt.Errorf("entry %d (offset %d): failed to read name length: %w", i+1, entryStartOffset, err)
		}
		currentOffset++ // Advance the current offset

		if nameLength == 0 {
			return fmt.Errorf("entry %d (offset %d): invalid name length (0)", i+1, entryStartOffset)
		}

		nameBytes := make([]byte, nameLength)
		nRead, err := io.ReadFull(u.vfsFile, nameBytes)
		currentOffset += int64(nRead)
		if err != nil {
			return fmt.Errorf("entry %d (offset %d): failed to read name (%d bytes): %w", i+1, entryStartOffset, nameLength, err)
		}
		fileName := string(nameBytes)
		// Normalize path separators
		fileName = strings.ReplaceAll(fileName, "\\", string(filepath.Separator))

		// Read file size and offset
		var fileSize uint32
		var fileOffset uint32
		metadataSuffixStartOffset := currentOffset // Position before reading fileSize/fileOffset/padding

		if err := binary.Read(u.vfsFile, binary.LittleEndian, &fileSize); err != nil {
			return fmt.Errorf("entry %d ('%s'): failed to read file size (offset %d): %w", i+1, fileName, metadataSuffixStartOffset, err)
		}
		currentOffset += 4 // Size of uint32

		if err := binary.Read(u.vfsFile, binary.LittleEndian, &fileOffset); err != nil {
			return fmt.Errorf("entry %d ('%s'): failed to read file offset (offset %d): %w", i+1, fileName, currentOffset, err)
		}
		currentOffset += 4 // Size of uint32

		// --- Validate File Data Offset and Size ---
		if int64(fileOffset) > u.vfsSize {
			return fmt.Errorf("entry %d ('%s'): invalid data offset %d (0x%X) - exceeds archive size %d", i+1, fileName, fileOffset, fileOffset, u.vfsSize)
		}
		if uint64(fileOffset)+uint64(fileSize) > uint64(u.vfsSize) {
			return fmt.Errorf("entry %d ('%s'): invalid data range - offset %d + size %d (%d) exceeds archive size %d", i+1, fileName, fileOffset, fileSize, uint64(fileOffset)+uint64(fileSize), u.vfsSize)
		}

		entry := VFSEntryMetadata{
			Name:       fileName,
			FileSize:   fileSize,
			FileOffset: fileOffset,
		}

		// --- Extract File Data ---
		if err := u.extractSingleFile(entry, i+1); err != nil {
			return err // Error already contains enough context
		}

		// --- Seek to Next Entry ---
		// After reading fileOffset, we need to skip the rest of the metadata block,
		// to reach the start of the next entry (nameLength).
		// Total size of the block after the name = entryFixedMetadataSuffixSize (16 bytes).
		// We have already read fileSize (4) and fileOffset (4), totaling 8 bytes.
		// Therefore, we need to skip another 16 - 8 = 8 bytes.
		bytesToSkip := entryFixedMetadataSuffixSize - 4 /*fileSize*/ - 4 /*fileOffset*/
		if bytesToSkip < 0 {
			// This situation should not occur with entryFixedMetadataSuffixSize = 16
			return fmt.Errorf("internal error: negative number of bytes to skip (%d)", bytesToSkip)
		}

		// Use relative seek to skip
		_, err = u.vfsFile.Seek(int64(bytesToSkip), io.SeekCurrent)
		if err != nil {
			// If EOF occurred while trying to skip after the last entry, it's normal.
			if err == io.EOF && i == u.header.FileCount-1 {
				fmt.Println("Reached end of file after processing metadata of the last entry.")
				break // Successfully processed the last file
			}
			return fmt.Errorf("entry %d ('%s'): failed to skip %d bytes to the next entry (current offset %d): %w", i+1, entry.Name, bytesToSkip, currentOffset, err)
		}
		currentOffset += int64(bytesToSkip) // Update the offset

	} // end of for loop over entries

	return nil
}

// extractSingleFile extracts the data of a single file based on its metadata.
func (u *Unpacker) extractSingleFile(entry VFSEntryMetadata, entryIndex uint32) error {
	// --- Extract File Data ---
	// Save the current position (where metadata is read) to return later
	metadataReadPos, err := u.vfsFile.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("entry %d ('%s'): failed to get current position before reading data: %w", entryIndex, entry.Name, err)
	}

	// Seek to the start of the file data
	dataStartPos, err := u.vfsFile.Seek(int64(entry.FileOffset), io.SeekStart)
	if err != nil {
		// This error is now unlikely due to the validation above, but check anyway
		return fmt.Errorf("entry %d ('%s'): failed to seek to data offset %d (0x%X): %w", entryIndex, entry.Name, entry.FileOffset, entry.FileOffset, err)
	}

	// Read the file data
	fileData := make([]byte, entry.FileSize)
	bytesRead, err := io.ReadFull(u.vfsFile, fileData)
	if err != nil {
		// Check for unexpected end of file, even after size checks
		if err == io.ErrUnexpectedEOF || (err == io.EOF && entry.FileSize > 0) {
			return fmt.Errorf("entry %d ('%s'): failed to read full data (started at offset %d [0x%X], expected size %d, read %d): unexpected end of file (VFS total size: %d) - archive might be corrupt: %w",
				entryIndex, entry.Name, dataStartPos, dataStartPos, entry.FileSize, bytesRead, u.vfsSize, err)
		}
		return fmt.Errorf("entry %d ('%s'): failed to read %d bytes of data from offset %d: %w", entryIndex, entry.Name, entry.FileSize, entry.FileOffset, err)
	}

	// --- Write Extracted File ---
	outputFilePath := filepath.Join(u.outputDir, entry.Name)

	// Ensure the directory for the file exists
	if err := os.MkdirAll(filepath.Dir(outputFilePath), 0755); err != nil {
		return fmt.Errorf("entry %d ('%s'): failed to create output directory '%s': %w", entryIndex, entry.Name, filepath.Dir(outputFilePath), err)
	}

	// Create and write the file
	outFile, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("entry %d ('%s'): failed to create output file '%s': %w", entryIndex, entry.Name, outputFilePath, err)
	}
	defer outFile.Close() // Ensure closure in any case

	_, writeErr := outFile.Write(fileData)
	if writeErr != nil {
		// Attempt to remove partially written file on error
		outFile.Close()           // Close first
		os.Remove(outputFilePath) // Then remove
		return fmt.Errorf("entry %d ('%s'): failed to write data to '%s': %w", entryIndex, entry.Name, outputFilePath, writeErr)
	}

	// Close the file explicitly here to check the close error
	closeErr := outFile.Close()
	if closeErr != nil {
		// Close error is less critical, but worth reporting
		fmt.Fprintf(os.Stderr, "Warning: failed to close output file '%s': %v\n", outputFilePath, closeErr)
	}

	fmt.Printf("Extracted (%d/%d): %s (%d bytes)\n", entryIndex, u.header.FileCount, entry.Name, entry.FileSize)

	// --- Return to the metadata reading position ---
	if _, err := u.vfsFile.Seek(metadataReadPos, io.SeekStart); err != nil {
		return fmt.Errorf("entry %d ('%s'): failed to return to metadata reading position (%d): %w", entryIndex, entry.Name, metadataReadPos, err)
	}

	return nil
}

// Unpack performs the complete unpacking process.
func (u *Unpacker) Unpack() error {
	// 1. Read and verify the header
	if err := u.readAndVerifyHeader(); err != nil {
		return err // Error already contains enough context
	}

	// If there are no files, exit
	if u.header.FileCount == 0 {
		return nil
	}

	// 2. Create the output directory (only if there are files)
	if err := u.createOutputDirectory(); err != nil {
		return err
	}

	// 3. Extract all files
	if err := u.extractFiles(); err != nil {
		return err
	}

	fmt.Println("Unpacking finished successfully.")
	return nil
}
