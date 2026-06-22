// Package actions performs the (reversible) cleanup operations. The MVP action
// is "move to trash". A Trasher abstracts the OS trash backend; the default is
// a freedesktop.org XDG Trash implementation (used on Linux and as a portable
// fallback elsewhere). An in-memory undo log allows reverting the last action.
package actions

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TrashedItem records one moved file for undo purposes.
type TrashedItem struct {
	Original string `json:"original"`
	Trashed  string `json:"trashed"` // path within the trash "files" dir
	infoFile string // path of the .trashinfo sidecar (internal)
}

// Result summarizes a trash action.
type Result struct {
	Trashed []TrashedItem     `json:"trashed"`
	Errors  map[string]string `json:"errors,omitempty"`
}

// Trasher moves a single path into the trash, returning the new location.
type Trasher interface {
	Trash(path string) (TrashedItem, error)
}

// Service executes cleanup actions and tracks an undo log.
type Service struct {
	trasher Trasher
	mu      sync.Mutex
	lastOp  []TrashedItem
}

// New returns a Service using the default trash backend for this OS.
func New() (*Service, error) {
	t, err := newXDGTrasher("")
	if err != nil {
		return nil, err
	}
	return &Service{trasher: t}, nil
}

// NewWithTrasher returns a Service using a custom Trasher (used in tests).
func NewWithTrasher(t Trasher) *Service { return &Service{trasher: t} }

// Trash moves each path to the trash, recording the batch for a later Undo.
// Read/verification of identity is the caller's responsibility (see
// internal/dupes.VerifyForAction); this method performs the move only.
func (s *Service) Trash(paths []string) Result {
	res := Result{Errors: map[string]string{}}
	var batch []TrashedItem
	for _, p := range paths {
		item, err := s.trasher.Trash(p)
		if err != nil {
			res.Errors[p] = err.Error()
			continue
		}
		batch = append(batch, item)
		res.Trashed = append(res.Trashed, item)
	}
	if len(res.Errors) == 0 {
		delete(res.Errors, "")
		res.Errors = nil
	}
	s.mu.Lock()
	s.lastOp = batch
	s.mu.Unlock()
	return res
}

// Undo restores the files moved by the most recent Trash call to their original
// locations. It returns the number restored and any per-path errors.
func (s *Service) Undo() (restored int, errs map[string]string) {
	s.mu.Lock()
	batch := s.lastOp
	s.lastOp = nil
	s.mu.Unlock()

	errs = map[string]string{}
	for _, item := range batch {
		if err := os.MkdirAll(filepath.Dir(item.Original), 0o755); err != nil {
			errs[item.Original] = err.Error()
			continue
		}
		if err := moveFile(item.Trashed, item.Original); err != nil {
			errs[item.Original] = err.Error()
			continue
		}
		if item.infoFile != "" {
			_ = os.Remove(item.infoFile)
		}
		restored++
	}
	if len(errs) == 0 {
		errs = nil
	}
	return restored, errs
}

// xdgTrasher implements the freedesktop.org Trash specification.
type xdgTrasher struct {
	filesDir string
	infoDir  string
}

// newXDGTrasher resolves the trash directory. If root is empty it uses
// $XDG_DATA_HOME/Trash (falling back to ~/.local/share/Trash).
func newXDGTrasher(root string) (*xdgTrasher, error) {
	if root == "" {
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			base = filepath.Join(home, ".local", "share")
		}
		root = filepath.Join(base, "Trash")
	}
	t := &xdgTrasher{
		filesDir: filepath.Join(root, "files"),
		infoDir:  filepath.Join(root, "info"),
	}
	for _, d := range []string{t.filesDir, t.infoDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, err
		}
	}
	return t, nil
}

func (t *xdgTrasher) Trash(path string) (TrashedItem, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return TrashedItem{}, err
	}
	if _, err := os.Lstat(abs); err != nil {
		return TrashedItem{}, err
	}

	name := uniqueName(t.filesDir, filepath.Base(abs))
	dest := filepath.Join(t.filesDir, name)
	infoPath := filepath.Join(t.infoDir, name+".trashinfo")

	// Write the info sidecar first so a partially-trashed file is recoverable.
	if err := os.WriteFile(infoPath, []byte(trashInfo(abs)), 0o600); err != nil {
		return TrashedItem{}, err
	}
	if err := moveFile(abs, dest); err != nil {
		_ = os.Remove(infoPath)
		return TrashedItem{}, err
	}
	return TrashedItem{Original: abs, Trashed: dest, infoFile: infoPath}, nil
}

// trashInfo renders a .trashinfo file body per the XDG spec.
func trashInfo(origAbs string) string {
	u := url.URL{Path: origAbs}
	return fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		u.EscapedPath(), time.Now().Format("2006-01-02T15:04:05"))
}

// uniqueName returns base, or base with a numeric suffix, that does not yet
// exist in dir.
func uniqueName(dir, base string) string {
	if _, err := os.Lstat(filepath.Join(dir, base)); os.IsNotExist(err) {
		return base
	}
	ext := filepath.Ext(base)
	stem := base[:len(base)-len(ext)]
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s_%d%s", stem, i, ext)
		if _, err := os.Lstat(filepath.Join(dir, cand)); os.IsNotExist(err) {
			return cand
		}
	}
}

// moveFile renames src to dst, falling back to copy+remove across filesystems.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}
