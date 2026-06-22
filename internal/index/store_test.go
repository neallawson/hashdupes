package index

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"hashdupes/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	// File-backed temp DB so WAL/pragmas behave as in production.
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestMigrateAndScanLifecycle(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	sc := &model.Scan{RootPath: "/tmp/root", Algo: "blake3", HeadBytes: 65536, Status: model.ScanRunning, StartedAt: time.Now()}
	id, err := st.CreateScan(ctx, sc)
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("expected non-zero scan id")
	}
	if err := st.FinishScan(ctx, id, model.ScanDone, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestCandidateFileGroups(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	sc := &model.Scan{RootPath: "/r", Algo: "sha256", HeadBytes: 65536, Status: model.ScanRunning, StartedAt: time.Now()}
	scanID, _ := st.CreateScan(ctx, sc)

	now := time.Now()
	hA := []byte{0x01, 0x02}
	hB := []byte{0x03, 0x04}
	files := []*model.File{
		{ScanID: scanID, Path: "/r/a1", Name: "a1", Size: 100, ModTime: now, HeadHash: hA},
		{ScanID: scanID, Path: "/r/a2", Name: "a2", Size: 100, ModTime: now, HeadHash: hA},
		{ScanID: scanID, Path: "/r/b1", Name: "b1", Size: 100, ModTime: now, HeadHash: hB}, // same size, diff head
		{ScanID: scanID, Path: "/r/uniq", Name: "uniq", Size: 999, ModTime: now, HeadHash: hA},
	}
	if err := st.InsertFiles(ctx, files); err != nil {
		t.Fatal(err)
	}

	groups, err := st.CandidateFileGroups(ctx, scanID)
	if err != nil {
		t.Fatal(err)
	}
	// Only (size=100, head=hA) has >1 member.
	if len(groups) != 1 {
		t.Fatalf("expected 1 candidate group, got %d", len(groups))
	}
	g := groups[0]
	if len(g.Files) != 2 {
		t.Fatalf("expected 2 files in group, got %d", len(g.Files))
	}
	if g.Reclaimable != 100 {
		t.Fatalf("expected reclaimable 100 (size*(n-1)), got %d", g.Reclaimable)
	}
}

func TestHashCacheRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	mt := time.Unix(0, 1234567890)
	want := []byte{0xde, 0xad, 0xbe, 0xef}

	if _, ok, err := st.CacheGet(ctx, "/r/x", 10, mt, "blake3", 65536); err != nil || ok {
		t.Fatalf("expected miss, got ok=%v err=%v", ok, err)
	}
	if err := st.CachePut(ctx, "/r/x", 10, mt, "blake3", 65536, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.CacheGet(ctx, "/r/x", 10, mt, "blake3", 65536)
	if err != nil || !ok {
		t.Fatalf("expected hit, got ok=%v err=%v", ok, err)
	}
	if string(got) != string(want) {
		t.Fatalf("cache value mismatch: got %x want %x", got, want)
	}
}
