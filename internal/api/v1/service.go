package v1

import (
	"context"
	"errors"
	"sync"

	"hashdupes/internal/dupes"
	"hashdupes/internal/hash"
	"hashdupes/internal/index"
	"hashdupes/internal/scan"
)

// Emitter delivers events to the frontend. The Wails-backed implementation
// lives in package main; tests provide a fake.
type Emitter interface {
	Emit(event string, data ...any)
}

// DirPicker presents a native directory chooser and returns the selected path
// (empty if the user cancels).
type DirPicker interface {
	PickDirectory(title string) (string, error)
}

// Service is the bound API object. Methods on it become callable from the
// frontend (read-only in this phase: no trash/move/delete).
type Service struct {
	store   *index.Store
	scanner *scan.Scanner
	dupes   *dupes.Service
	emit    Emitter
	picker  DirPicker

	mu     sync.Mutex
	cancel context.CancelFunc // non-nil while a scan is running
}

// New constructs the v1 Service.
func New(store *index.Store, emit Emitter, picker DirPicker) *Service {
	return &Service{
		store:   store,
		scanner: scan.New(store),
		dupes:   dupes.New(store),
		emit:    emit,
		picker:  picker,
	}
}

// ChooseFolder opens a native directory picker and returns the chosen path.
func (s *Service) ChooseFolder() (string, error) {
	return s.picker.PickDirectory("Select a folder to scan")
}

// StartScan launches a scan in the background and returns immediately. Progress
// and completion are reported via events (EventScanProgress/Done/Error). Only
// one scan may run at a time.
func (s *Service) StartScan(req ScanRequest) error {
	if req.RootPath == "" {
		return errors.New("rootPath is required")
	}
	algo := hash.Algo(req.Algo)
	if req.Algo != "" && !algo.Valid() {
		return errors.New("invalid hash algorithm: " + req.Algo)
	}

	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return errors.New("a scan is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.mu.Unlock()

	opts := scan.Options{
		Root:           req.RootPath,
		Algo:           algo,
		HeadBytes:      req.HeadBytes,
		Workers:        req.Workers,
		FollowSymlinks: req.FollowSymlinks,
		IgnoreHidden:   req.IgnoreHidden,
	}

	go func() {
		defer func() {
			s.mu.Lock()
			s.cancel = nil
			s.mu.Unlock()
		}()

		res, err := s.scanner.Run(ctx, opts, func(p scan.Progress) {
			s.emit.Emit(EventScanProgress, p)
		})
		if err != nil && ctx.Err() == nil {
			s.emit.Emit(EventScanError, ErrorDTO{Message: err.Error()})
			return
		}
		s.emit.Emit(EventScanDone, ScanDoneDTO{
			ScanID:      res.ScanID,
			FilesHashed: res.FilesHashed,
			FoldersSeen: res.FoldersSeen,
			Skipped:     res.Skipped,
		})
	}()
	return nil
}

// CancelScan requests cancellation of the running scan, if any.
func (s *Service) CancelScan() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// GetReport returns the lazy (unverified) duplicate report for a scan: file
// groups are candidates pending byte-compare confirmation via VerifyGroup.
func (s *Service) GetReport(scanID int64) (ReportDTO, error) {
	rep, err := s.dupes.CandidateReport(context.Background(), scanID)
	if err != nil {
		return ReportDTO{}, err
	}
	return toReportDTO(scanID, rep), nil
}

// VerifyGroup byte-compares the paths of a candidate group and returns the
// confirmed duplicate subgroups. This is the on-demand verification invoked
// when the user expands a group.
func (s *Service) VerifyGroup(req VerifyRequest) (VerifyResultDTO, error) {
	if len(req.Paths) < 2 {
		return VerifyResultDTO{}, nil
	}
	confirmed, errs := dupes.ConfirmPaths(req.Size, req.Paths)

	out := VerifyResultDTO{Groups: make([]FileGroupDTO, len(confirmed))}
	for i, g := range confirmed {
		out.Groups[i] = toConfirmedFileGroupDTO(g)
	}
	if len(errs) > 0 {
		out.Errors = make(map[string]string, len(errs))
		for p, e := range errs {
			out.Errors[p] = e.Error()
		}
	}
	return out, nil
}
