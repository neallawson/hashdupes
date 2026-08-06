// Package dupes turns raw candidate groups from the index into confirmed
// duplicate groups. File candidates (matching size + head hash) are confirmed
// by a mandatory byte-by-byte comparison; folder duplicates come from the
// content-only Merkle hash. It also computes subsumption: file groups that live
// entirely inside duplicated folders are flagged so the UI can collapse them.
package dupes

import (
	"context"
	"os"
	"strings"

	"hashdupes/internal/index"
	"hashdupes/internal/model"
	"hashdupes/internal/verify"
)

// FileGroup is a confirmed duplicate-file group plus subsumption metadata.
type FileGroup struct {
	model.FileDupGroup
	// WithinDupFolder is true when every file in the group lives under a folder
	// that is itself duplicated, so the group is redundant with a folder group.
	WithinDupFolder bool `json:"withinDupFolder"`
}

// Report is the full duplicate picture for a scan.
type Report struct {
	FolderGroups []model.FolderDupGroup `json:"folderGroups"`
	FileGroups   []FileGroup            `json:"fileGroups"`
	VerifyErrors map[string]string      `json:"verifyErrors,omitempty"`
}

// Service derives duplicate reports from a Store.
type Service struct {
	store *index.Store
}

// New returns a Service backed by store.
func New(store *index.Store) *Service { return &Service{store: store} }

// Report builds the confirmed duplicate report for a scan.
func (s *Service) Report(ctx context.Context, scanID int64) (Report, error) {
	folderGroups, err := s.store.FolderDupGroups(ctx, scanID)
	if err != nil {
		return Report{}, err
	}
	candidates, err := s.store.CandidateFileGroups(ctx, scanID)
	if err != nil {
		return Report{}, err
	}

	dupFolderPaths := folderPaths(folderGroups)
	verifyErrs := map[string]string{}
	var fileGroups []FileGroup

	for _, cand := range candidates {
		confirmed, errs := ConfirmGroup(cand)
		for p, e := range errs {
			verifyErrs[p] = e.Error()
		}
		for _, g := range confirmed {
			fileGroups = append(fileGroups, FileGroup{
				FileDupGroup:    g,
				WithinDupFolder: groupWithinDupFolder(g, dupFolderPaths),
			})
		}
	}

	rep := Report{FolderGroups: folderGroups, FileGroups: fileGroups}
	if len(verifyErrs) > 0 {
		rep.VerifyErrors = verifyErrs
	}
	return rep, nil
}

// CandidateReport builds an *unverified* report: file groups are raw
// candidates (matching size + head hash) and are not byte-compared. This backs
// the lazy verification model, where confirmation happens per-group on demand
// via ConfirmPaths. Folder groups and subsumption flags are still computed.
func (s *Service) CandidateReport(ctx context.Context, scanID int64) (Report, error) {
	folderGroups, err := s.store.FolderDupGroups(ctx, scanID)
	if err != nil {
		return Report{}, err
	}
	candidates, err := s.store.CandidateFileGroups(ctx, scanID)
	if err != nil {
		return Report{}, err
	}
	dupFolderPaths := folderPaths(folderGroups)
	fileGroups := make([]FileGroup, 0, len(candidates))
	for _, cand := range candidates {
		fileGroups = append(fileGroups, FileGroup{
			FileDupGroup:    cand, // Verified stays false
			WithinDupFolder: groupWithinDupFolder(cand, dupFolderPaths),
		})
	}
	return Report{FolderGroups: folderGroups, FileGroups: fileGroups}, nil
}

// ConfirmPaths is the lazy, on-demand verification entry point used by the API.
// Given the paths of a candidate group (all sharing size), it byte-compares
// them and returns the confirmed duplicate subgroups.
func ConfirmPaths(size int64, paths []string) ([]model.FileDupGroup, map[string]error) {
	cand := model.FileDupGroup{Size: size}
	for _, p := range paths {
		cand.Files = append(cand.Files, model.File{Path: p, Size: size})
	}
	return ConfirmGroup(cand)
}

// ConfirmGroup splits a single candidate group into one or more confirmed
// groups by byte-comparing members. Singletons (whose only true match dropped
// out) are discarded. This is the lazy verification used both for display and
// before any destructive action.
func ConfirmGroup(cand model.FileDupGroup) (groups []model.FileDupGroup, errs map[string]error) {
	byPath := make(map[string]model.File, len(cand.Files))
	paths := make([]string, 0, len(cand.Files))
	for _, f := range cand.Files {
		byPath[f.Path] = f
		paths = append(paths, f.Path)
	}

	parts, perrs := verify.PartitionByContent(paths)
	for _, part := range parts {
		if len(part) < 2 {
			continue // no longer a duplicate once verified
		}
		g := model.FileDupGroup{Size: cand.Size, HeadHash: cand.HeadHash, Verified: true}
		for _, p := range part {
			g.Files = append(g.Files, byPath[p])
		}
		g.Reclaimable = g.Size * int64(len(g.Files)-1)
		groups = append(groups, g)
	}
	return groups, perrs
}

// VerifyForAction confirms that the given paths are byte-identical before a
// destructive action. It returns the subset of paths that are confirmed equal
// to the first path (the surviving "keep" candidate set), and any read errors.
// No path that fails verification is included.
func VerifyForAction(keep string, candidates []string) (confirmed []string, errs map[string]error) {
	errs = map[string]error{}
	for _, p := range candidates {
		if p == keep {
			continue
		}
		eq, err := verify.EqualFiles(keep, p)
		if err != nil {
			errs[p] = err
			continue
		}
		if eq {
			confirmed = append(confirmed, p)
		}
	}
	return confirmed, errs
}

// folderPaths collects every duplicated folder's path (cleaned, with trailing
// separator) for prefix testing.
func folderPaths(groups []model.FolderDupGroup) []string {
	var out []string
	for _, g := range groups {
		for _, f := range g.Folders {
			out = append(out, ensureTrailingSep(f.Path))
		}
	}
	return out
}

func ensureTrailingSep(p string) string {
	if strings.HasSuffix(p, string(os.PathSeparator)) {
		return p
	}
	return p + string(os.PathSeparator)
}

// groupWithinDupFolder reports whether every file in g lives under one of the
// duplicated folder paths.
func groupWithinDupFolder(g model.FileDupGroup, dupFolderPaths []string) bool {
	if len(dupFolderPaths) == 0 {
		return false
	}
	for _, f := range g.Files {
		if !pathUnderAny(f.Path, dupFolderPaths) {
			return false
		}
	}
	return true
}

func pathUnderAny(path string, prefixes []string) bool {
	for _, pre := range prefixes {
		if strings.HasPrefix(path, pre) {
			return true
		}
	}
	return false
}
