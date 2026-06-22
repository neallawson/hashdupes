package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"hashdupes/internal/hash"
	"hashdupes/internal/index"
)

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// buildFixture creates a tree with a known duplicate file and a pair of
// duplicate folders (same contents, different names).
func buildFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "a.txt"), []byte("hello"))
	write(t, filepath.Join(root, "copy_of_a.txt"), []byte("hello")) // dup file
	write(t, filepath.Join(root, "unique.txt"), []byte("different"))

	// dirX and dirY have identical contents (content-only folder dup).
	write(t, filepath.Join(root, "dirX", "f1"), []byte("one"))
	write(t, filepath.Join(root, "dirX", "f2"), []byte("two"))
	write(t, filepath.Join(root, "dirY", "f1"), []byte("one"))
	write(t, filepath.Join(root, "dirY", "f2"), []byte("two"))

	write(t, filepath.Join(root, "empty.txt"), nil) // zero-byte file
	return root
}

func TestScanDetectsDuplicates(t *testing.T) {
	ctx := context.Background()
	root := buildFixture(t)

	st, err := index.Open(ctx, filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	sc := New(st)
	res, err := sc.Run(ctx, Options{Root: root, Algo: hash.AlgoSHA256, Workers: 4}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	groups, err := st.CandidateFileGroups(ctx, res.ScanID)
	if err != nil {
		t.Fatal(err)
	}
	// Expect the "hello" pair as a candidate group of 2.
	var found bool
	for _, g := range groups {
		if len(g.Files) == 2 && g.Size == int64(len("hello")) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a 2-file candidate group for the duplicate 'hello' files; got %d groups", len(groups))
	}

	folderGroups, err := st.FolderDupGroups(ctx, res.ScanID)
	if err != nil {
		t.Fatal(err)
	}
	var dupFolders bool
	for _, fg := range folderGroups {
		for _, f := range fg.Folders {
			if f.Name == "dirX" || f.Name == "dirY" {
				dupFolders = true
			}
		}
	}
	if !dupFolders {
		t.Fatal("expected dirX and dirY to be detected as duplicate folders")
	}
}

func TestScanCacheReuse(t *testing.T) {
	ctx := context.Background()
	root := buildFixture(t)
	st, err := index.Open(ctx, filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	sc := New(st)
	r1, err := sc.Run(ctx, Options{Root: root, Algo: hash.AlgoSHA256}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r1.FilesHashed == 0 {
		t.Fatal("first scan should hash files")
	}
	// Second scan: every file fingerprint is cached, so bytesHashed should be 0.
	r2State := &runState{scanner: sc, opts: Options{Root: root, Algo: hash.AlgoSHA256, HeadBytes: hash.DefaultHeadBytes, Workers: 2}}
	tree, err := r2State.build(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := r2State.hashFiles(ctx, tree); err != nil {
		t.Fatal(err)
	}
	if r2State.bytesHashed != 0 {
		t.Fatalf("expected 0 bytes hashed on cached rescan, got %d", r2State.bytesHashed)
	}
}
