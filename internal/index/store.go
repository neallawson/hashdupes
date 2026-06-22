// Package index is the persistence layer for hashdupes. It wraps an embedded
// SQLite database (pure-Go modernc.org/sqlite driver), owns schema migrations
// (via goose), and exposes the queries used to record scans and surface
// duplicate candidates.
package index

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	"hashdupes/internal/model"

	_ "modernc.org/sqlite" // register the "sqlite" database/sql driver
)

// driverName is the database/sql driver registered by modernc.org/sqlite.
const driverName = "sqlite"

// Store is a handle to the hashdupes database.
type Store struct {
	db *sql.DB
}

// Open opens (creating if necessary) the SQLite database at path, applies
// pragmas for safe concurrent local use, and runs all pending migrations.
// Use ":memory:" for an ephemeral in-process database (tests).
func Open(ctx context.Context, path string) (*Store, error) {
	dsn := buildDSN(path)
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("index: open db: %w", err)
	}
	// A single writer avoids "database is locked" under WAL for our workload;
	// reads still proceed concurrently at the SQLite level.
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("index: ping db: %w", err)
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// buildDSN renders a modernc.org/sqlite DSN with our standard pragmas.
func buildDSN(path string) string {
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(NORMAL)")
	return "file:" + path + "?" + q.Encode()
}

// DB exposes the underlying *sql.DB for tooling (e.g. migration commands).
func (s *Store) DB() *sql.DB { return s.db }

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// ---- time helpers ------------------------------------------------------

func toNano(t time.Time) int64 { return t.UnixNano() }

func fromNano(n int64) time.Time { return time.Unix(0, n) }

func nullNano(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.UnixNano(), Valid: true}
}

func ptrFromNull(n sql.NullInt64) *time.Time {
	if !n.Valid {
		return nil
	}
	t := time.Unix(0, n.Int64)
	return &t
}

// ---- scans -------------------------------------------------------------

// CreateScan inserts a new scan row and returns its ID.
func (s *Store) CreateScan(ctx context.Context, sc *model.Scan) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO scans (root_path, algo, head_bytes, status, started_at)
		 VALUES (?,?,?,?,?)`,
		sc.RootPath, sc.Algo, sc.HeadBytes, string(sc.Status), toNano(sc.StartedAt))
	if err != nil {
		return 0, fmt.Errorf("index: create scan: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	sc.ID = id
	return id, nil
}

// FinishScan updates a scan's terminal status and finish time.
func (s *Store) FinishScan(ctx context.Context, id int64, status model.ScanStatus, finishedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE scans SET status=?, finished_at=? WHERE id=?`,
		string(status), toNano(finishedAt), id)
	if err != nil {
		return fmt.Errorf("index: finish scan: %w", err)
	}
	return nil
}

// ---- folders & files ---------------------------------------------------

// InsertFolder inserts a folder and assigns its generated ID back onto f.
func (s *Store) InsertFolder(ctx context.Context, f *model.Folder) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO folders (scan_id, parent_id, path, name, folder_hash, is_empty, file_count, total_size)
		 VALUES (?,?,?,?,?,?,?,?)`,
		f.ScanID, nullID(f.ParentID), f.Path, f.Name, f.FolderHash, boolInt(f.IsEmpty), f.FileCount, f.TotalSize)
	if err != nil {
		return fmt.Errorf("index: insert folder: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	f.ID = id
	return nil
}

// InsertFiles inserts a batch of files within a single transaction.
func (s *Store) InsertFiles(ctx context.Context, files []*model.File) error {
	if len(files) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO files (scan_id, parent_id, path, name, size, mtime, ctime, ext, is_empty, head_hash, hashed_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, f := range files {
		if _, err := stmt.ExecContext(ctx,
			f.ScanID, nullID(f.ParentID), f.Path, f.Name, f.Size,
			toNano(f.ModTime), nullNano(f.ChangeTime), f.Ext, boolInt(f.IsEmpty),
			f.HeadHash, nullNano(f.HashedAt)); err != nil {
			return fmt.Errorf("index: insert file %q: %w", f.Path, err)
		}
	}
	return tx.Commit()
}

// ---- hash cache --------------------------------------------------------

// CacheGet returns the cached head hash for a file fingerprint, if present.
func (s *Store) CacheGet(ctx context.Context, path string, size int64, mtime time.Time, algo string, headBytes int64) ([]byte, bool, error) {
	var h []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT head_hash FROM hash_cache
		 WHERE path=? AND size=? AND mtime=? AND algo=? AND head_bytes=?`,
		path, size, toNano(mtime), algo, headBytes).Scan(&h)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return h, true, nil
}

// CachePut upserts a head-hash cache entry.
func (s *Store) CachePut(ctx context.Context, path string, size int64, mtime time.Time, algo string, headBytes int64, headHash []byte) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO hash_cache (path, size, mtime, algo, head_bytes, head_hash)
		 VALUES (?,?,?,?,?,?)
		 ON CONFLICT(path, size, mtime, algo, head_bytes)
		 DO UPDATE SET head_hash=excluded.head_hash`,
		path, size, toNano(mtime), algo, headBytes, headHash)
	return err
}

// ---- duplicate queries -------------------------------------------------

// CandidateFileGroups returns groups of files sharing (size, head_hash). These
// are *candidates*; callers must confirm true equality via package verify
// before treating them as duplicates or acting on them.
func (s *Store) CandidateFileGroups(ctx context.Context, scanID int64) ([]model.FileDupGroup, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT f.id, f.scan_id, f.parent_id, f.path, f.name, f.size, f.mtime, f.ctime, f.ext, f.is_empty, f.head_hash
		FROM files f
		JOIN (
			SELECT size, head_hash
			FROM files
			WHERE scan_id=? AND head_hash IS NOT NULL
			GROUP BY size, head_hash
			HAVING COUNT(*) > 1
		) d ON f.size=d.size AND f.head_hash=d.head_hash
		WHERE f.scan_id=?
		ORDER BY f.size, f.head_hash, f.path`, scanID, scanID)
	if err != nil {
		return nil, fmt.Errorf("index: candidate file groups: %w", err)
	}
	defer rows.Close()

	var groups []model.FileDupGroup
	var cur *model.FileDupGroup
	for rows.Next() {
		var (
			f        model.File
			parent   sql.NullInt64
			ctime    sql.NullInt64
			mtimeN   int64
		)
		if err := rows.Scan(&f.ID, &f.ScanID, &parent, &f.Path, &f.Name, &f.Size, &mtimeN, &ctime, &f.Ext, &f.IsEmpty, &f.HeadHash); err != nil {
			return nil, err
		}
		f.ParentID = parent.Int64
		f.ModTime = fromNano(mtimeN)
		f.ChangeTime = ptrFromNull(ctime)

		if cur == nil || cur.Size != f.Size || !bytesEqual(cur.HeadHash, f.HeadHash) {
			groups = append(groups, model.FileDupGroup{Size: f.Size, HeadHash: f.HeadHash})
			cur = &groups[len(groups)-1]
		}
		cur.Files = append(cur.Files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range groups {
		groups[i].Reclaimable = groups[i].Size * int64(len(groups[i].Files)-1)
	}
	return groups, nil
}

// FolderDupGroups returns groups of folders sharing the same content-only
// folder hash.
func (s *Store) FolderDupGroups(ctx context.Context, scanID int64) ([]model.FolderDupGroup, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT fo.id, fo.scan_id, fo.parent_id, fo.path, fo.name, fo.folder_hash, fo.is_empty, fo.file_count, fo.total_size
		FROM folders fo
		JOIN (
			SELECT folder_hash
			FROM folders
			WHERE scan_id=? AND folder_hash IS NOT NULL
			GROUP BY folder_hash
			HAVING COUNT(*) > 1
		) d ON fo.folder_hash=d.folder_hash
		WHERE fo.scan_id=?
		ORDER BY fo.folder_hash, fo.path`, scanID, scanID)
	if err != nil {
		return nil, fmt.Errorf("index: folder dup groups: %w", err)
	}
	defer rows.Close()

	var groups []model.FolderDupGroup
	var cur *model.FolderDupGroup
	for rows.Next() {
		var (
			fo     model.Folder
			parent sql.NullInt64
		)
		if err := rows.Scan(&fo.ID, &fo.ScanID, &parent, &fo.Path, &fo.Name, &fo.FolderHash, &fo.IsEmpty, &fo.FileCount, &fo.TotalSize); err != nil {
			return nil, err
		}
		fo.ParentID = parent.Int64
		if cur == nil || !bytesEqual(cur.FolderHash, fo.FolderHash) {
			groups = append(groups, model.FolderDupGroup{FolderHash: fo.FolderHash})
			cur = &groups[len(groups)-1]
		}
		cur.Folders = append(cur.Folders, fo)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range groups {
		if n := len(groups[i].Folders); n > 1 {
			groups[i].Reclaimable = groups[i].Folders[0].TotalSize * int64(n-1)
		}
	}
	return groups, nil
}

// ---- small helpers -----------------------------------------------------

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullID(id int64) sql.NullInt64 {
	if id == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: id, Valid: true}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
