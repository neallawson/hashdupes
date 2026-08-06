// Package v1 is the versioned application API exposed to the frontend. It is
// intentionally decoupled from Wails: event emission and folder selection are
// provided via the Emitter and DirPicker interfaces, whose Wails-backed
// implementations live in package main. A future v2 can live alongside this
// package without breaking existing bindings.
package v1

import (
	"encoding/hex"
	"strconv"
	"time"

	"hashdupes/internal/dupes"
	"hashdupes/internal/model"
)

// Version identifies this API surface.
const Version = "v1"

// Event names emitted to the frontend during a scan.
const (
	EventScanProgress = "scan:progress"
	EventScanDone     = "scan:done"
	EventScanError    = "scan:error"
)

// ScanRequest is the payload for StartScan.
type ScanRequest struct {
	RootPath       string `json:"rootPath"`
	Algo           string `json:"algo"`      // "blake3" | "sha256"; empty => default
	HeadBytes      int64  `json:"headBytes"` // 0 => default (64 KiB)
	Workers        int    `json:"workers"`   // 0 => NumCPU
	FollowSymlinks bool   `json:"followSymlinks"`
	IgnoreHidden   bool   `json:"ignoreHidden"`
}

// ScanDoneDTO is the payload of the scan:done event.
type ScanDoneDTO struct {
	ScanID      int64 `json:"scanId"`
	FilesHashed int64 `json:"filesHashed"`
	FoldersSeen int64 `json:"foldersSeen"`
	Skipped     int64 `json:"skipped"`
}

// ErrorDTO is the payload of the scan:error event.
type ErrorDTO struct {
	Message string `json:"message"`
}

// FileDTO is a single file in a duplicate group.
type FileDTO struct {
	ID      int64  `json:"id"`
	Path    string `json:"path"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"` // RFC3339
	Ext     string `json:"ext"`
	IsEmpty bool   `json:"isEmpty"`
}

// FileGroupDTO is a duplicate-file group (candidate or confirmed).
type FileGroupDTO struct {
	ID              string    `json:"id"` // stable key: hexHead-size
	Size            int64     `json:"size"`
	HeadHash        string    `json:"headHash"`
	Files           []FileDTO `json:"files"`
	Reclaimable     int64     `json:"reclaimable"`
	Verified        bool      `json:"verified"`
	WithinDupFolder bool      `json:"withinDupFolder"`
}

// FolderDTO is a single folder in a duplicate group.
type FolderDTO struct {
	ID         int64  `json:"id"`
	Path       string `json:"path"`
	Name       string `json:"name"`
	FolderHash string `json:"folderHash"`
	IsEmpty    bool   `json:"isEmpty"`
	FileCount  int64  `json:"fileCount"`
	TotalSize  int64  `json:"totalSize"`
}

// FolderGroupDTO is a duplicate-folder group.
type FolderGroupDTO struct {
	ID          string      `json:"id"`
	FolderHash  string      `json:"folderHash"`
	Folders     []FolderDTO `json:"folders"`
	Reclaimable int64       `json:"reclaimable"`
}

// ReportDTO is the full duplicate report for a scan.
type ReportDTO struct {
	ScanID       int64            `json:"scanId"`
	FolderGroups []FolderGroupDTO `json:"folderGroups"`
	FileGroups   []FileGroupDTO   `json:"fileGroups"`
	// AllVerified is false for the lazy candidate report; individual groups
	// carry their own Verified flag.
	AllVerified bool `json:"allVerified"`
}

// VerifyRequest asks the backend to byte-compare a candidate group's paths.
type VerifyRequest struct {
	Size  int64    `json:"size"`
	Paths []string `json:"paths"`
}

// VerifyResultDTO returns the confirmed subgroups for a verified candidate.
type VerifyResultDTO struct {
	Groups []FileGroupDTO    `json:"groups"`
	Errors map[string]string `json:"errors,omitempty"`
}

// ---- conversions -------------------------------------------------------

func groupID(headHash []byte, size int64) string {
	return hex.EncodeToString(headHash) + "-" + strconv.FormatInt(size, 10)
}

func toFileDTO(f model.File) FileDTO {
	return FileDTO{
		ID:      f.ID,
		Path:    f.Path,
		Name:    f.Name,
		Size:    f.Size,
		ModTime: f.ModTime.Format(time.RFC3339),
		Ext:     f.Ext,
		IsEmpty: f.IsEmpty,
	}
}

func toFileGroupDTO(g dupes.FileGroup) FileGroupDTO {
	files := make([]FileDTO, len(g.Files))
	for i, f := range g.Files {
		files[i] = toFileDTO(f)
	}
	return FileGroupDTO{
		ID:              groupID(g.HeadHash, g.Size),
		Size:            g.Size,
		HeadHash:        hex.EncodeToString(g.HeadHash),
		Files:           files,
		Reclaimable:     g.Reclaimable,
		Verified:        g.Verified,
		WithinDupFolder: g.WithinDupFolder,
	}
}

func toConfirmedFileGroupDTO(g model.FileDupGroup) FileGroupDTO {
	files := make([]FileDTO, len(g.Files))
	for i, f := range g.Files {
		files[i] = toFileDTO(f)
	}
	return FileGroupDTO{
		ID:          groupID(g.HeadHash, g.Size),
		Size:        g.Size,
		HeadHash:    hex.EncodeToString(g.HeadHash),
		Files:       files,
		Reclaimable: g.Reclaimable,
		Verified:    g.Verified,
	}
}

func toFolderGroupDTO(g model.FolderDupGroup) FolderGroupDTO {
	folders := make([]FolderDTO, len(g.Folders))
	for i, f := range g.Folders {
		folders[i] = FolderDTO{
			ID:         f.ID,
			Path:       f.Path,
			Name:       f.Name,
			FolderHash: hex.EncodeToString(f.FolderHash),
			IsEmpty:    f.IsEmpty,
			FileCount:  f.FileCount,
			TotalSize:  f.TotalSize,
		}
	}
	return FolderGroupDTO{
		ID:          hex.EncodeToString(g.FolderHash),
		FolderHash:  hex.EncodeToString(g.FolderHash),
		Folders:     folders,
		Reclaimable: g.Reclaimable,
	}
}

func toReportDTO(scanID int64, rep dupes.Report) ReportDTO {
	fg := make([]FileGroupDTO, len(rep.FileGroups))
	for i, g := range rep.FileGroups {
		fg[i] = toFileGroupDTO(g)
	}
	fo := make([]FolderGroupDTO, len(rep.FolderGroups))
	for i, g := range rep.FolderGroups {
		fo[i] = toFolderGroupDTO(g)
	}
	return ReportDTO{ScanID: scanID, FolderGroups: fo, FileGroups: fg}
}
