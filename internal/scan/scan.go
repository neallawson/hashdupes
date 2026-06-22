// Package scan walks a directory tree, head-hashes its files in parallel
// (reusing the persistent hash cache), computes content-only folder Merkle
// hashes bottom-up, and persists the result as a scan in the index store.
package scan

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"hashdupes/internal/hash"
	"hashdupes/internal/index"
	"hashdupes/internal/model"

	"golang.org/x/sync/errgroup"
)

// Options configures a single scan run.
type Options struct {
	Root           string
	Algo           hash.Algo
	HeadBytes      int64
	Workers        int  // 0 => runtime.NumCPU()
	FollowSymlinks bool // default false: symlinks are skipped
	IgnoreHidden   bool // skip dot-files/dot-dirs
}

func (o *Options) withDefaults() {
	if o.Algo == "" {
		o.Algo = hash.DefaultAlgo
	}
	if o.HeadBytes == 0 {
		o.HeadBytes = hash.DefaultHeadBytes
	}
	if o.Workers <= 0 {
		o.Workers = defaultWorkers()
	}
}

// Progress is a snapshot emitted during a scan.
type Progress struct {
	FilesSeen   int64  `json:"filesSeen"`
	FilesHashed int64  `json:"filesHashed"`
	BytesHashed int64  `json:"bytesHashed"`
	Skipped     int64  `json:"skipped"`
	CurrentPath string `json:"currentPath"`
	Done        bool   `json:"done"`
}

// ProgressFunc receives throttled progress updates. It may be nil.
type ProgressFunc func(Progress)

// Result summarizes a completed scan.
type Result struct {
	ScanID      int64
	FilesHashed int64
	FoldersSeen int64
	Skipped     int64
}

// Scanner runs scans against a Store.
type Scanner struct {
	store *index.Store
}

// New returns a Scanner backed by store.
func New(store *index.Store) *Scanner { return &Scanner{store: store} }

// Run executes a full scan: walk, hash, folder-hash, and persist. The returned
// Result references the persisted scan. Run is cancellable via ctx; on cancel
// the scan row is marked canceled and the partial tree is still persisted.
func (s *Scanner) Run(ctx context.Context, opts Options, progress ProgressFunc) (Result, error) {
	opts.withDefaults()

	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return Result{}, err
	}

	sc := &model.Scan{
		RootPath:  root,
		Algo:      string(opts.Algo),
		HeadBytes: opts.HeadBytes,
		Status:    model.ScanRunning,
		StartedAt: time.Now(),
	}
	scanID, err := s.store.CreateScan(ctx, sc)
	if err != nil {
		return Result{}, err
	}

	r := &runState{
		scanner: s,
		opts:    opts,
		scanID:  scanID,
		onProg:  progress,
	}

	runErr := r.execute(ctx, root)

	status := model.ScanDone
	if ctx.Err() != nil {
		status = model.ScanCanceled
	} else if runErr != nil {
		status = model.ScanError
	}
	if ferr := s.store.FinishScan(ctx, scanID, status, time.Now()); ferr != nil && runErr == nil {
		runErr = ferr
	}
	r.emit(true)

	return Result{
		ScanID:      scanID,
		FilesHashed: atomic.LoadInt64(&r.filesHashed),
		FoldersSeen: r.foldersSeen,
		Skipped:     atomic.LoadInt64(&r.skipped),
	}, runErr
}

// runState carries mutable state for a single Run.
type runState struct {
	scanner *Scanner
	opts    Options
	scanID  int64
	onProg  ProgressFunc

	filesSeen   int64
	filesHashed int64
	bytesHashed int64
	skipped     int64
	foldersSeen int64

	lastEmit atomic.Int64 // unix-nano of last progress emit
	curPath  atomic.Value // string
}

func (r *runState) execute(ctx context.Context, root string) error {
	tree, err := r.build(ctx, root)
	if err != nil {
		return err
	}
	if err := r.hashFiles(ctx, tree); err != nil {
		return err
	}
	computeFolderHashes(tree, r.opts.Algo)
	return r.persist(ctx, tree)
}

// emit invokes the progress callback with the current counters (throttled
// unless done is true).
func (r *runState) emit(done bool) {
	if r.onProg == nil {
		return
	}
	now := time.Now().UnixNano()
	const throttle = int64(100 * time.Millisecond)
	if !done && now-r.lastEmit.Load() < throttle {
		return
	}
	r.lastEmit.Store(now)
	cur, _ := r.curPath.Load().(string)
	r.onProg(Progress{
		FilesSeen:   atomic.LoadInt64(&r.filesSeen),
		FilesHashed: atomic.LoadInt64(&r.filesHashed),
		BytesHashed: atomic.LoadInt64(&r.bytesHashed),
		Skipped:     atomic.LoadInt64(&r.skipped),
		CurrentPath: cur,
		Done:        done,
	})
}

func defaultWorkers() int {
	n := runtime.NumCPU()
	if n < 1 {
		return 1
	}
	return n
}

func isHidden(name string) bool {
	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}

// statTimes returns the modification time and (best-effort) change time.
// Change time is platform-dependent and currently returned as nil; the field
// exists so a per-OS implementation can be added without schema changes.
func statTimes(info os.FileInfo) (mtime time.Time, ctime *time.Time) {
	return info.ModTime(), nil
}

// errgroupLimited builds an errgroup with a concurrency limit.
func errgroupLimited(ctx context.Context, limit int) (*errgroup.Group, context.Context) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)
	return g, gctx
}
