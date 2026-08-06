package v1

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"hashdupes/internal/index"
)

// fakeEmitter records events and signals scan completion.
type fakeEmitter struct {
	done chan ScanDoneDTO
	errs chan ErrorDTO
}

func newFakeEmitter() *fakeEmitter {
	return &fakeEmitter{done: make(chan ScanDoneDTO, 1), errs: make(chan ErrorDTO, 1)}
}

func (e *fakeEmitter) Emit(event string, data ...any) {
	if len(data) == 0 {
		return
	}
	switch event {
	case EventScanDone:
		if d, ok := data[0].(ScanDoneDTO); ok {
			e.done <- d
		}
	case EventScanError:
		if d, ok := data[0].(ErrorDTO); ok {
			e.errs <- d
		}
	}
}

type fakePicker struct{ path string }

func (p fakePicker) PickDirectory(string) (string, error) { return p.path, nil }

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestServiceScanReportVerify(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a.txt"), []byte("hello"))
	write(t, filepath.Join(root, "b.txt"), []byte("hello"))     // true dup of a
	write(t, filepath.Join(root, "c.txt"), []byte("world"))     // unique

	st, err := index.Open(t.Context(), filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	emit := newFakeEmitter()
	svc := New(st, emit, fakePicker{path: root})

	// ChooseFolder returns the fake path.
	if p, err := svc.ChooseFolder(); err != nil || p != root {
		t.Fatalf("ChooseFolder=%q err=%v", p, err)
	}

	if err := svc.StartScan(ScanRequest{RootPath: root, Algo: "sha256"}); err != nil {
		t.Fatalf("StartScan: %v", err)
	}

	var done ScanDoneDTO
	select {
	case done = <-emit.done:
	case e := <-emit.errs:
		t.Fatalf("scan error event: %s", e.Message)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for scan:done")
	}

	rep, err := svc.GetReport(done.ScanID)
	if err != nil {
		t.Fatal(err)
	}
	// Lazy report: at least one candidate group, none marked verified yet.
	if len(rep.FileGroups) == 0 {
		t.Fatal("expected at least one candidate file group")
	}
	var hello FileGroupDTO
	for _, g := range rep.FileGroups {
		if g.Verified {
			t.Fatal("candidate report groups must not be pre-verified")
		}
		if g.Size == int64(len("hello")) && len(g.Files) == 2 {
			hello = g
		}
	}
	if hello.ID == "" {
		t.Fatal("expected the 2-file 'hello' candidate group")
	}

	// Verify on demand -> confirmed.
	paths := []string{hello.Files[0].Path, hello.Files[1].Path}
	vr, err := svc.VerifyGroup(VerifyRequest{Size: hello.Size, Paths: paths})
	if err != nil {
		t.Fatal(err)
	}
	if len(vr.Groups) != 1 || !vr.Groups[0].Verified || len(vr.Groups[0].Files) != 2 {
		t.Fatalf("expected one confirmed group of 2, got %+v", vr.Groups)
	}
}

func TestStartScanRejectsEmptyRoot(t *testing.T) {
	st, err := index.Open(t.Context(), filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := New(st, newFakeEmitter(), fakePicker{})
	if err := svc.StartScan(ScanRequest{}); err == nil {
		t.Fatal("expected error for empty rootPath")
	}
}
