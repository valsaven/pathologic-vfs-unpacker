# pathologic-vfs-unpacker

![demo](https://github.com/valsaven/pathologic-vfs-unpacker/blob/master/demo.gif)

`pathologic-vfs-unpacker` is a command-line tool written in Go to unpack VFS archives from the _Pathologic_ game series (e.g., _Pathologic Classic HD_).

It extracts files from the game's VFS format, making it useful for modders, data explorers or fans wanting to dive into the game's assets.

## Features

- Unpacks VFS archives with the `LP1C` magic bytes (specific to _Pathologic_).
- **Fast unpacking**: Significantly outperforms other tools, extracting archives (e.g., Geometries.vfs) in seconds, where others may take tens of minutes (even on high-end PC).
- Supports nested file paths with proper directory creation.
- Cross-platform: works on Windows, Linux and macOS.
- Simple and lightweight, built with Go.

## Installation

### Prerequisites

- [Go](https://golang.org/dl/) (version 1.24 or higher recommended).

### Build from Source

1. Clone the repository:

   ```shell
   git clone https://github.com/[your-username]/pathologic-vfs-unpacker.git
   cd pathologic-vfs-unpacker
   ```

2. Build the executable:

   - **Windows:**

   ```shell
   go build -o pathologic-vfs-unpacker.exe
   ```

   - **Linux/macOS:**

   ```shell
   go build -o pathologic-vfs-unpacker
   ```

3. (Optional) Move the binary to a directory in your `PATH` for global access:

   ```shell
   # Linux/macOS example
   mv pathologic-vfs-unpacker /usr/local/bin/
   ```

## Usage

Run the tool with the following syntax:

```text
pathologic-vfs-unpacker <path_to_vfs_file> [output_directory]
```

- `<path_to_vfs_file>`: Path to the `.vfs` file you want to unpack (required).
- `[output_directory]`: Directory where extracted files will be saved (optional). If omitted, it defaults to a folder named after the VFS file (without the `.vfs` extension) in the current directory.

### Examples

1. **Unpack to default directory:**

   ```shell
   pathologic-vfs-unpacker.exe "D:\Steam\steamapps\common\Pathologic Classic HD\data\Actors.vfs"
   ```

   - Creates a folder named `Actors` in the current directory and extracts files there.

2. **Unpack to a custom directory:**

   ```shell
   pathologic-vfs-unpacker.exe Actors.vfs ./extracted_files
   ```

   - Extracts all files from `Actors.vfs` into the `./extracted_files` directory.

3. **Full path with spaces (use quotes):**

   ```shell
   pathologic-vfs-unpacker "/path/to/My Archive.vfs" "/home/user/pathologic_assets"
   ```

### Output

The tool will:

- Verify the VFS file's magic bytes (`LP1C`).
- Display the number of files in the archive.
- Extract each file, preserving its internal directory structure (e.g., `Textures\file.dds`).
- Show progress like:

  ```text
  Archive contains 42 files.
  Extracted (1/42): Textures\stone.dds (12345 bytes)
  Extracted (2/42): Scripts\dialogue.txt (678 bytes)
  ...
  Unpacking finished successfully.
  ```

### Error Handling

If something goes wrong (e.g., corrupt file, invalid format), the tool provides detailed error messages:

```text
Error during unpacking: file 3 ('Textures\broken.dds'): failed to read 1024 bytes of data from offset 2048: unexpected end of file
```

## Supported Formats

Currently, the tool supports VFS files with the `LP1C` header, as used in _Pathologic Classic HD_.
Support for other versions (e.g., _Pathologic 2_) may require adjustments—feel free to contribute!

## Contributing

Contributions are welcome! If you encounter bugs, have feature suggestions, or want to add support for other VFS variants:

1. Fork the repository.
2. Create a feature branch (`git checkout -b feature/name`).
3. Submit a pull request.

Please include detailed descriptions of changes and test cases if possible.

## License

This project is licensed under the [MIT License](LICENSE).
