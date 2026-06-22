package dupes

import (
	"os"
	"path/filepath"
	"testing"

	"hashdupes/internal/model"
)

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConfirmGroupSplitsOnContent(t *testing.T) {
	dir := t.TempDir()
	// Same size + (pretend) same head, but two distinct contents.
	a1 := filepath.Join(dir, "a1")
	a2 := filepath.Join(dir, "a2")
	b1 := filepath.Join(dir, "b1")
	write(t, a1, []byte("AAAA1111"))
	write(t, a2, []byte("AAAA1111"))
	write(t, b1, []byte("AAAA2222"))

	cand := model.FileDupGroup{
		Size:     8,
		HeadHash: []byte{0xaa},
		Files: []model.File{
			{Path: a1, Size: 8}, {Path: a2, Size: 8}, {Path: b1, Size: 8},
		},
	}
	groups, errs := ConfirmGroup(cand)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// b1 is alone after verification, so it drops; only the a-group survives.
	if len(groups) != 1 {
		t.Fatalf("expected 1 confirmed group, got %d", len(groups))
	}
	if len(groups[0].Files) != 2 {
		t.Fatalf("expected 2 confirmed files, got %d", len(groups[0].Files))
	}
	if !groups[0].Verified {
		t.Fatal("confirmed group should be marked Verified")
	}
	if groups[0].Reclaimable != 8 {
		t.Fatalf("expected reclaimable 8, got %d", groups[0].Reclaimable)
	}
}

func TestVerifyForAction(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep")
	same := filepath.Join(dir, "same")
	diff := filepath.Join(dir, "diff")
	write(t, keep, []byte("data"))
	write(t, same, []byte("data"))
	write(t, diff, []byte("nope"))

	confirmed, errs := VerifyForAction(keep, []string{keep, same, diff})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(confirmed) != 1 || confirmed[0] != same {
		t.Fatalf("expected only %q confirmed, got %v", same, confirmed)
	}
}

func TestGroupWithinDupFolder(t *testing.T) {
	dupFolders := []string{ensureTrailingSep("/root/dirX"), ensureTrailingSep("/root/dirY")}
	inside := model.FileDupGroup{Files: []model.File{
		{Path: "/root/dirX/f1"}, {Path: "/root/dirY/f1"},
	}}
	outside := model.FileDupGroup{Files: []model.File{
		{Path: "/root/dirX/f1"}, {Path: "/root/other/f1"},
	}}
	if !groupWithinDupFolder(inside, dupFolders) {
		t.Fatal("expected inside group to be flagged within dup folder")
	}
	if groupWithinDupFolder(outside, dupFolders) {
		t.Fatal("did not expect outside group to be flagged")
	}
}
