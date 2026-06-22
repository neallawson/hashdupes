package scan

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"hashdupes/internal/hash"
	"hashdupes/internal/model"
)

// fileNode is an in-memory representation of a file discovered during the walk.
type fileNode struct {
	path    string
	name    string
	ext     string
	size    int64
	mtime   time.Time
	ctime   *time.Time
	isEmpty bool

	headHash []byte // filled during the hashing phase
}

// dirNode is an in-memory directory with its children and aggregates.
type dirNode struct {
	path string
	name string

	parent *dirNode
	dirs   []*dirNode
	files  []*fileNode

	folderHash []byte
	fileCount  int64 // recursive count of descendant files
	totalSize  int64 // recursive sum of descendant file sizes

	id int64 // assigned at persist time
}

// build walks the tree rooted at root, constructing the dirNode/fileNode graph.
// Symlinks are skipped unless opts.FollowSymlinks; dot entries are skipped when
// opts.IgnoreHidden. Permission and transient errors skip the offending entry
// and increment the skipped counter rather than aborting the scan.
func (r *runState) build(ctx context.Context, root string) (*dirNode, error) {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !rootInfo.IsDir() {
		return nil, &fs.PathError{Op: "scan", Path: root, Err: fs.ErrInvalid}
	}

	rootNode := &dirNode{path: root, name: filepath.Base(root)}
	dirs := map[string]*dirNode{root: rootNode}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			atomic.AddInt64(&r.skipped, 1)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		name := d.Name()
		if r.opts.IgnoreHidden && isHidden(name) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		// Symlink handling: skip by default to avoid cycles/double counting.
		if d.Type()&fs.ModeSymlink != 0 && !r.opts.FollowSymlinks {
			atomic.AddInt64(&r.skipped, 1)
			return nil
		}

		parent := dirs[filepath.Dir(path)]
		if parent == nil {
			// Parent was skipped; skip descendants too.
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			node := &dirNode{path: path, name: name, parent: parent}
			parent.dirs = append(parent.dirs, node)
			dirs[path] = node
			r.foldersSeen++
			return nil
		}

		info, ierr := d.Info()
		if ierr != nil {
			atomic.AddInt64(&r.skipped, 1)
			return nil
		}
		if !info.Mode().IsRegular() {
			atomic.AddInt64(&r.skipped, 1)
			return nil
		}
		mtime, ctime := statTimes(info)
		fn := &fileNode{
			path:    path,
			name:    name,
			ext:     filepath.Ext(name),
			size:    info.Size(),
			mtime:   mtime,
			ctime:   ctime,
			isEmpty: info.Size() == 0,
		}
		parent.files = append(parent.files, fn)
		atomic.AddInt64(&r.filesSeen, 1)
		return nil
	})

	r.foldersSeen++ // count the root itself
	if walkErr != nil && ctx.Err() == nil {
		return rootNode, walkErr
	}
	return rootNode, nil
}

// collectFiles flattens all fileNodes in the tree.
func collectFiles(root *dirNode) []*fileNode {
	var out []*fileNode
	var walk func(d *dirNode)
	walk = func(d *dirNode) {
		out = append(out, d.files...)
		for _, sub := range d.dirs {
			walk(sub)
		}
	}
	walk(root)
	return out
}

// hashFiles computes the head hash of every file in parallel, reusing the
// persistent cache keyed by (path, size, mtime, algo, head_bytes).
func (r *runState) hashFiles(ctx context.Context, root *dirNode) error {
	files := collectFiles(root)
	g, gctx := errgroupLimited(ctx, r.opts.Workers)

	var mu sync.Mutex // serializes cache writes on the single-conn store
	algoName := string(r.opts.Algo)

	for _, fn := range files {
		fn := fn
		g.Go(func() error {
			if gctx.Err() != nil {
				return gctx.Err()
			}
			r.curPath.Store(fn.path)

			if h, ok, err := r.scanner.store.CacheGet(gctx, fn.path, fn.size, fn.mtime, algoName, r.opts.HeadBytes); err == nil && ok {
				fn.headHash = h
			} else {
				h, herr := hash.HeadHashFile(fn.path, fn.size, r.opts.HeadBytes, r.opts.Algo)
				if herr != nil {
					atomic.AddInt64(&r.skipped, 1)
					r.emit(false)
					return nil // skip unreadable file, do not fail the scan
				}
				fn.headHash = h
				mu.Lock()
				_ = r.scanner.store.CachePut(gctx, fn.path, fn.size, fn.mtime, algoName, r.opts.HeadBytes, h)
				mu.Unlock()
				atomic.AddInt64(&r.bytesHashed, minInt64(fn.size, r.opts.HeadBytes))
			}
			atomic.AddInt64(&r.filesHashed, 1)
			r.emit(false)
			return nil
		})
	}
	return g.Wait()
}

// computeFolderHashes fills folderHash, fileCount, and totalSize for every
// directory using a post-order (bottom-up) traversal.
func computeFolderHashes(root *dirNode, algo hash.Algo) {
	var visit func(d *dirNode)
	visit = func(d *dirNode) {
		var children []hash.Child
		for _, f := range d.files {
			d.fileCount++
			d.totalSize += f.size
			children = append(children, hash.Child{Hash: f.headHash, Size: f.size, IsDir: false})
		}
		for _, sub := range d.dirs {
			visit(sub)
			d.fileCount += sub.fileCount
			d.totalSize += sub.totalSize
			children = append(children, hash.Child{Hash: sub.folderHash, IsDir: true})
		}
		d.folderHash, _ = hash.FolderHash(children, algo)
	}
	visit(root)
}

// persist writes folders (top-down, to resolve parent IDs) and then all files
// in a batch.
func (r *runState) persist(ctx context.Context, root *dirNode) error {
	var files []*model.File

	var walk func(d *dirNode, parentID int64) error
	walk = func(d *dirNode, parentID int64) error {
		mf := &model.Folder{
			ScanID:     r.scanID,
			ParentID:   parentID,
			Path:       d.path,
			Name:       d.name,
			FolderHash: d.folderHash,
			IsEmpty:    len(d.dirs) == 0 && len(d.files) == 0,
			FileCount:  d.fileCount,
			TotalSize:  d.totalSize,
		}
		if err := r.scanner.store.InsertFolder(ctx, mf); err != nil {
			return err
		}
		d.id = mf.ID
		now := time.Now()
		for _, f := range d.files {
			files = append(files, &model.File{
				ScanID:     r.scanID,
				ParentID:   d.id,
				Path:       f.path,
				Name:       f.name,
				Size:       f.size,
				ModTime:    f.mtime,
				ChangeTime: f.ctime,
				Ext:        f.ext,
				IsEmpty:    f.isEmpty,
				HeadHash:   f.headHash,
				HashedAt:   &now,
			})
		}
		for _, sub := range d.dirs {
			if err := walk(sub, d.id); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root, 0); err != nil {
		return err
	}
	return r.scanner.store.InsertFiles(ctx, files)
}

func minInt64(a, b int64) int64 {
	if b <= 0 || a < b {
		return a
	}
	return b
}
