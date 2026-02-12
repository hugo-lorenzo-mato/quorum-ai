package snapshot

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ValidateSnapshot verifies archive structure and file checksums and returns the manifest.
func ValidateSnapshot(inputPath string) (*Manifest, error) {
	manifest, _, err := loadSnapshotArchive(inputPath)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

type archivedFile struct {
	Path string
	Data []byte
	Mode int64
}

func loadSnapshotArchive(inputPath string) (*Manifest, map[string]archivedFile, error) {
	if inputPath == "" {
		return nil, nil, fmt.Errorf("input path is required")
	}

	archiveFiles, err := readArchiveFiles(inputPath)
	if err != nil {
		return nil, nil, err
	}

	manifestFile, ok := archiveFiles[manifestArchivePath]
	if !ok {
		return nil, nil, fmt.Errorf("snapshot is missing %s", manifestArchivePath)
	}

	manifest, err := decodeManifest(manifestFile.Data)
	if err != nil {
		return nil, nil, fmt.Errorf("decoding manifest: %w", err)
	}

	if err := validateArchiveAgainstManifest(manifest, archiveFiles); err != nil {
		return nil, nil, err
	}

	return manifest, archiveFiles, nil
}

func readArchiveFiles(inputPath string) (map[string]archivedFile, error) {
	file, err := os.Open(inputPath) // #nosec G304 -- caller controls path
	if err != nil {
		return nil, fmt.Errorf("opening snapshot: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("opening gzip stream: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	files := make(map[string]archivedFile)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar entry: %w", err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg, tar.TypeRegA: //nolint:staticcheck // TypeRegA kept for backward compat with older archives
			// Regular files are expected.
		default:
			return nil, fmt.Errorf("unsupported tar entry type %d for %s", header.Typeflag, header.Name)
		}

		entryPath, cleanErr := cleanArchivePath(filepath.ToSlash(header.Name))
		if cleanErr != nil {
			return nil, fmt.Errorf("invalid archive path %q: %w", header.Name, cleanErr)
		}

		data, readErr := io.ReadAll(tarReader)
		if readErr != nil {
			return nil, fmt.Errorf("reading tar entry %s: %w", entryPath, readErr)
		}

		files[entryPath] = archivedFile{
			Path: entryPath,
			Data: data,
			Mode: header.Mode,
		}
	}

	return files, nil
}

func validateArchiveAgainstManifest(manifest *Manifest, archiveFiles map[string]archivedFile) error {
	for _, fileEntry := range manifest.Files {
		archiveFile, ok := archiveFiles[fileEntry.Path]
		if !ok {
			return fmt.Errorf("manifest entry not found in archive: %s", fileEntry.Path)
		}

		if int64(len(archiveFile.Data)) != fileEntry.Size {
			return fmt.Errorf("size mismatch for %s: manifest=%d archive=%d", fileEntry.Path, fileEntry.Size, len(archiveFile.Data))
		}

		hash := sha256.Sum256(archiveFile.Data)
		if hex.EncodeToString(hash[:]) != fileEntry.SHA256 {
			return fmt.Errorf("checksum mismatch for %s", fileEntry.Path)
		}
	}

	if _, ok := archiveFiles[registryArchivePath]; !ok {
		return fmt.Errorf("snapshot is missing required entry: %s", registryArchivePath)
	}
	if manifest.GlobalConfigPresent {
		if _, ok := archiveFiles[globalConfigArchivePath]; !ok {
			return fmt.Errorf("snapshot is missing required global config entry: %s", globalConfigArchivePath)
		}
	}

	return nil
}
