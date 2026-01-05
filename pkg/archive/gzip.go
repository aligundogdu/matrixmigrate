package archive

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SaveGzipJSON saves data as gzipped JSON
func SaveGzipJSON(filePath string, data interface{}) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	// Encode JSON
	encoder := json.NewEncoder(gzWriter)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// LoadGzipJSON loads gzipped JSON data
func LoadGzipJSON(filePath string, data interface{}) error {
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Decode JSON
	decoder := json.NewDecoder(gzReader)
	if err := decoder.Decode(data); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	return nil
}

// CompressFile compresses a file with gzip
func CompressFile(srcPath, dstPath string) error {
	// Open source file
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(dst)
	defer gzWriter.Close()

	// Copy data
	if _, err := io.Copy(gzWriter, src); err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}

	return nil
}

// DecompressFile decompresses a gzip file
func DecompressFile(srcPath, dstPath string) error {
	// Open source file
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create destination file
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy data
	if _, err := io.Copy(dst, gzReader); err != nil {
		return fmt.Errorf("failed to decompress data: %w", err)
	}

	return nil
}

// GetUncompressedSize returns the uncompressed size of a gzip file
func GetUncompressedSize(filePath string) (int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return 0, err
	}
	defer gzReader.Close()

	// Read all to get size (not ideal for large files)
	data, err := io.ReadAll(gzReader)
	if err != nil {
		return 0, err
	}

	return int64(len(data)), nil
}


