package attachments

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// OwnerType identifies the namespace for attachments.
// It is also used as a directory name under .quorum/attachments/.
type OwnerType string

const (
	OwnerChatSession OwnerType = "chat"
	OwnerWorkflow    OwnerType = "workflows"
)

const (
	// MaxAttachmentSizeBytes limits each uploaded attachment size.
	// Keep in sync with frontend hints.
	MaxAttachmentSizeBytes = 50 * 1024 * 1024 // 50MB
)

type Store struct {
	root    string
	baseDir string
}

func NewStore(root string) *Store {
	baseDir := filepath.Join(root, ".quorum", "attachments")
	return &Store{root: root, baseDir: baseDir}
}

func (s *Store) BaseDir() string {
	return s.baseDir
}

// Root returns the project root directory used by this store.
func (s *Store) Root() string {
	return s.root
}

func (s *Store) EnsureBaseDir() error {
	return os.MkdirAll(s.baseDir, 0o750)
}

func (s *Store) Save(ownerType OwnerType, ownerID string, r io.Reader, filename string) (core.Attachment, error) {
	if err := s.validateOwner(ownerType, ownerID); err != nil {
		return core.Attachment{}, err
	}
	if err := s.EnsureBaseDir(); err != nil {
		return core.Attachment{}, fmt.Errorf("ensuring base dir: %w", err)
	}

	attachmentID := uuid.New().String()
	safeName := sanitizeFilename(filename)

	root, err := os.OpenRoot(s.baseDir)
	if err != nil {
		return core.Attachment{}, fmt.Errorf("opening attachments root: %w", err)
	}
	defer func() { _ = root.Close() }()

	attachmentDirRel := filepath.Join(string(ownerType), ownerID, attachmentID)
	if err := root.MkdirAll(attachmentDirRel, 0o750); err != nil {
		return core.Attachment{}, fmt.Errorf("creating attachment dir: %w", err)
	}

	fileRelPath := filepath.Join(attachmentDirRel, safeName)
	f, err := root.OpenFile(fileRelPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return core.Attachment{}, fmt.Errorf("creating attachment file: %w", err)
	}
	defer f.Close()

	// Sniff content-type from the first 512 bytes.
	var sniffBuf [512]byte
	n, _ := io.ReadFull(r, sniffBuf[:])
	if n > 0 {
		if _, err := f.Write(sniffBuf[:n]); err != nil {
			return core.Attachment{}, fmt.Errorf("writing attachment header: %w", err)
		}
	}
	contentType := http.DetectContentType(sniffBuf[:n])

	remainingLimit := int64(MaxAttachmentSizeBytes - n)
	if remainingLimit < 0 {
		return core.Attachment{}, fmt.Errorf("attachment too large (max %d bytes)", MaxAttachmentSizeBytes)
	}
	written, err := io.Copy(f, io.LimitReader(r, remainingLimit+1))
	if err != nil {
		return core.Attachment{}, fmt.Errorf("writing attachment: %w", err)
	}
	if written > remainingLimit {
		return core.Attachment{}, fmt.Errorf("attachment too large (max %d bytes)", MaxAttachmentSizeBytes)
	}

	size := int64(n) + written

	metaRelPath := filepath.ToSlash(filepath.Join(".quorum", "attachments", string(ownerType), ownerID, attachmentID, safeName))
	meta := core.Attachment{
		ID:          attachmentID,
		Name:        safeName,
		Path:        metaRelPath,
		Size:        size,
		ContentType: contentType,
		CreatedAt:   time.Now(),
	}

	metaPath := filepath.Join(attachmentDirRel, "meta.json")
	if err := writeJSONFile(root, metaPath, meta, 0o600); err != nil {
		return core.Attachment{}, fmt.Errorf("writing meta: %w", err)
	}

	return meta, nil
}

func (s *Store) List(ownerType OwnerType, ownerID string) ([]core.Attachment, error) {
	if err := s.validateOwner(ownerType, ownerID); err != nil {
		return nil, err
	}

	dir := filepath.Join(s.baseDir, string(ownerType), ownerID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []core.Attachment{}, nil
		}
		return nil, fmt.Errorf("reading owner dir: %w", err)
	}

	var out []core.Attachment
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		metaPath := filepath.Join(dir, ent.Name(), "meta.json")
		b, err := os.ReadFile(metaPath) // #nosec G304 -- controlled path within .quorum/attachments
		if err != nil {
			continue
		}
		var meta core.Attachment
		if err := json.Unmarshal(b, &meta); err != nil {
			continue
		}
		out = append(out, meta)
	}

	return out, nil
}

func (s *Store) Resolve(ownerType OwnerType, ownerID, attachmentID string) (core.Attachment, string, error) {
	if err := s.validateOwner(ownerType, ownerID); err != nil {
		return core.Attachment{}, "", err
	}
	if strings.TrimSpace(attachmentID) == "" {
		return core.Attachment{}, "", fmt.Errorf("attachment id is required")
	}

	dir := filepath.Join(s.baseDir, string(ownerType), ownerID, attachmentID)
	metaPath := filepath.Join(dir, "meta.json")
	b, err := os.ReadFile(metaPath) // #nosec G304 -- controlled path within .quorum/attachments
	if err != nil {
		if os.IsNotExist(err) {
			return core.Attachment{}, "", os.ErrNotExist
		}
		return core.Attachment{}, "", fmt.Errorf("reading meta: %w", err)
	}

	var meta core.Attachment
	if err := json.Unmarshal(b, &meta); err != nil {
		return core.Attachment{}, "", fmt.Errorf("parsing meta: %w", err)
	}
	abs := filepath.Join(s.root, filepath.FromSlash(meta.Path))
	return meta, abs, nil
}

func (s *Store) Delete(ownerType OwnerType, ownerID, attachmentID string) error {
	if err := s.validateOwner(ownerType, ownerID); err != nil {
		return err
	}
	if strings.TrimSpace(attachmentID) == "" {
		return fmt.Errorf("attachment id is required")
	}

	dir := filepath.Join(s.baseDir, string(ownerType), ownerID, attachmentID)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing attachment: %w", err)
	}
	return nil
}

func (s *Store) DeleteAll(ownerType OwnerType, ownerID string) error {
	if err := s.validateOwner(ownerType, ownerID); err != nil {
		return err
	}
	dir := filepath.Join(s.baseDir, string(ownerType), ownerID)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing owner attachments: %w", err)
	}
	return nil
}

func (s *Store) validateOwner(ownerType OwnerType, ownerID string) error {
	if ownerType != OwnerChatSession && ownerType != OwnerWorkflow {
		return fmt.Errorf("invalid owner type: %q", ownerType)
	}
	if strings.TrimSpace(ownerID) == "" {
		return fmt.Errorf("owner id is required")
	}
	return nil
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)

	// First, replace backslashes with underscores if they're not path separators
	// On Windows, backslashes are path separators and filepath.Base handles them
	// But if the input comes from a Unix system, backslashes might be literal characters
	// We normalize by replacing backslashes before extracting base
	if strings.Contains(name, "\\") {
		// Replace backslashes with underscores
		name = strings.ReplaceAll(name, "\\", "_")
		// Don't call filepath.Base since we already handled the path structure
		base := name
		base = strings.ReplaceAll(base, "\x00", "")
		base = strings.ReplaceAll(base, "/", "_")
		if base == "" || base == "." || base == ".." {
			base = "attachment"
		}
		const maxLen = 200
		if len(base) > maxLen {
			base = base[:maxLen]
		}
		return base
	}

	// No backslashes - use normal filepath.Base to extract filename from Unix paths
	base := filepath.Base(name)
	base = strings.TrimSpace(base)
	base = strings.ReplaceAll(base, "\x00", "")
	base = strings.ReplaceAll(base, string(os.PathSeparator), "_")
	base = strings.ReplaceAll(base, "/", "_")
	if base == "" || base == "." || base == ".." {
		base = "attachment"
	}
	const maxLen = 200
	if len(base) > maxLen {
		base = base[:maxLen]
	}
	return base
}

func writeJSONFile(root *os.Root, path string, v interface{}, perm os.FileMode) error {
	tmp := path + ".tmp"
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := root.WriteFile(tmp, b, perm); err != nil {
		return err
	}
	return root.Rename(tmp, path)
}
