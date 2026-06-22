package verify

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEqualFiles(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", []byte("hello world"))
	b := writeFile(t, dir, "b", []byte("hello world"))
	c := writeFile(t, dir, "c", []byte("hello worlD"))      // 1 byte diff
	d := writeFile(t, dir, "d", []byte("hello world extra")) // length diff

	cases := []struct {
		x, y string
		want bool
	}{
		{a, b, true},
		{a, c, false},
		{a, d, false},
		{a, a, true},
	}
	for _, tc := range cases {
		got, err := EqualFiles(tc.x, tc.y)
		if err != nil {
			t.Fatalf("EqualFiles(%s,%s): %v", tc.x, tc.y, err)
		}
		if got != tc.want {
			t.Fatalf("EqualFiles(%s,%s)=%v want %v", tc.x, tc.y, got, tc.want)
		}
	}
}

func TestPartitionByContent(t *testing.T) {
	dir := t.TempDir()
	// Two distinct contents, sharing the same length so size pre-filtering
	// would not separate them: simulates a head-collision candidate group.
	a1 := writeFile(t, dir, "a1", []byte("AAAA1111"))
	a2 := writeFile(t, dir, "a2", []byte("AAAA1111"))
	a3 := writeFile(t, dir, "a3", []byte("AAAA1111"))
	b1 := writeFile(t, dir, "b1", []byte("AAAA2222")) // same head "AAAA", diff tail
	b2 := writeFile(t, dir, "b2", []byte("AAAA2222"))

	groups, errs := PartitionByContent([]string{a1, b1, a2, b2, a3})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 content subgroups, got %d: %v", len(groups), groups)
	}
	// Normalize and verify membership.
	sizes := []int{len(groups[0]), len(groups[1])}
	sort.Ints(sizes)
	if sizes[0] != 2 || sizes[1] != 3 {
		t.Fatalf("expected subgroup sizes {2,3}, got %v", sizes)
	}
}

func TestPartitionMissingFile(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a", []byte("x"))
	missing := filepath.Join(dir, "nope")

	groups, errs := PartitionByContent([]string{a, missing})
	if _, ok := errs[missing]; !ok {
		t.Fatal("expected an error recorded for the missing file")
	}
	// The missing file must never appear in a group.
	for _, g := range groups {
		for _, p := range g {
			if p == missing {
				t.Fatal("missing file should be excluded from all groups")
			}
		}
	}
}
