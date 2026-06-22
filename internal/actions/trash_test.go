package actions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrashAndUndo(t *testing.T) {
	work := t.TempDir()
	trashRoot := t.TempDir()

	orig := filepath.Join(work, "dup.txt")
	if err := os.WriteFile(orig, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	tr, err := newXDGTrasher(trashRoot)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewWithTrasher(tr)

	res := svc.Trash([]string{orig})
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	if len(res.Trashed) != 1 {
		t.Fatalf("expected 1 trashed item, got %d", len(res.Trashed))
	}
	if _, err := os.Stat(orig); !os.IsNotExist(err) {
		t.Fatal("original file should be gone after trashing")
	}
	// Info sidecar exists.
	if _, err := os.Stat(filepath.Join(trashRoot, "info", "dup.txt.trashinfo")); err != nil {
		t.Fatalf("expected .trashinfo sidecar: %v", err)
	}

	restored, errs := svc.Undo()
	if errs != nil {
		t.Fatalf("undo errors: %v", errs)
	}
	if restored != 1 {
		t.Fatalf("expected 1 restored, got %d", restored)
	}
	if data, err := os.ReadFile(orig); err != nil || string(data) != "payload" {
		t.Fatalf("file not properly restored: data=%q err=%v", data, err)
	}
}

func TestUniqueNameCollision(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got := uniqueName(dir, "a.txt")
	if got != "a_1.txt" {
		t.Fatalf("expected a_1.txt, got %q", got)
	}
}
