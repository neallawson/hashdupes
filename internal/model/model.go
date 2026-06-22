// Package model defines the core domain types shared across the hashdupes
// backend. These types are storage- and transport-agnostic; persistence
// concerns live in package index and the wire/DTO shapes live in internal/api.
package model

import "time"

// ScanStatus is the lifecycle state of a scan session.
type ScanStatus string

const (
	ScanRunning  ScanStatus = "running"
	ScanDone     ScanStatus = "done"
	ScanError    ScanStatus = "error"
	ScanCanceled ScanStatus = "canceled"
)

// Scan is a single recursive indexing session over a root directory.
type Scan struct {
	ID         int64
	RootPath   string
	Algo       string // hash algorithm name, e.g. "blake3"
	HeadBytes  int64  // number of leading bytes hashed per file
	Status     ScanStatus
	StartedAt  time.Time
	FinishedAt *time.Time
}

// File is an indexed regular file.
//
// HeadHash is always populated (hash of the first HeadBytes of content, or the
// whole file if smaller). Duplicate identity is the (Size, HeadHash) pair;
// true equality is confirmed lazily by a byte-by-byte comparison, never by the
// head hash alone.
type File struct {
	ID       int64
	ScanID   int64
	ParentID int64 // owning folder ID (0 if root-level)
	Path     string
	Name     string
	Size     int64
	ModTime  time.Time
	// ChangeTime is best-effort and platform-dependent; nil when unavailable.
	ChangeTime *time.Time
	Ext        string
	IsEmpty    bool // Size == 0
	HeadHash   []byte
	HashedAt   *time.Time
}

// Folder is an indexed directory. FolderHash is a content-only Merkle hash
// derived from the sorted hashes of its children (see package hash).
type Folder struct {
	ID         int64
	ScanID     int64
	ParentID   int64 // owning folder ID (0 if root)
	Path       string
	Name       string
	FolderHash []byte
	IsEmpty    bool // no non-skipped children
	FileCount  int64
	TotalSize  int64
}

// DupKind distinguishes the two kinds of duplicate groups.
type DupKind string

const (
	DupFile   DupKind = "file"
	DupFolder DupKind = "folder"
)

// FileDupGroup is a set of files that share (Size, HeadHash) and have been (or
// will be) confirmed identical by byte comparison.
type FileDupGroup struct {
	Size       int64
	HeadHash   []byte
	Files      []File
	Verified   bool // true once byte-compare has confirmed membership
	Reclaimable int64 // bytes recoverable if all but one copy are removed
}

// FolderDupGroup is a set of folders sharing the same content-only Merkle hash.
type FolderDupGroup struct {
	FolderHash  []byte
	Folders     []Folder
	Reclaimable int64
}
