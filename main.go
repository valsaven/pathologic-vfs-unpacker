package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Handle command-line arguments
	if len(os.Args) < 2 || len(os.Args) > 3 {
		printUsage()
		os.Exit(1)
	}

	vfsPath := filepath.Clean(os.Args[1])
	outputDir := ""

	if len(os.Args) == 3 {
		outputDir = filepath.Clean(os.Args[2])
	} else {
		// Default output directory name: VFS filename without extension
		baseName := filepath.Base(vfsPath)
		outputDir = strings.TrimSuffix(baseName, filepath.Ext(baseName))
		// Ensure the path is clean, even if generated
		outputDir = filepath.Clean(outputDir)
	}

	fmt.Printf("Input VFS: %s\n", vfsPath)
	fmt.Printf("Output Directory: %s\n", outputDir)

	// Create and configure the unpacker
	unpacker, err := NewUnpacker(vfsPath, outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nInitialization error: %v\n", err)
		os.Exit(1)
	}
	// Ensure VFS file is closed on main exit
	defer unpacker.Close()

	// Start unpacking
	err = unpacker.Unpack()
	if err != nil {
		// Error should already be informative from Unpacker methods
		fmt.Fprintf(os.Stderr, "\nError during unpacking: %v\n", err)
		os.Exit(1) // Exit with error code
	}
}

// printUsage prints program usage information.
func printUsage() {
	appName := filepath.Base(os.Args[0])

	fmt.Printf("Usage: %s <path_to_vfs_file> [output_directory]\n", appName)
	fmt.Println("If output_directory is not specified, a directory named after")
	fmt.Println("the VFS file (without extension) in the current location is used.")
	fmt.Println("\nExamples:")
	fmt.Printf("  %s \"D:\\Steam\\steamapps\\common\\Pathologic Classic HD\\data\\Sounds.vfs\"\n", appName) // Example path
	fmt.Printf("  %s Sounds.vfs extracted_sounds\n", appName)
}
