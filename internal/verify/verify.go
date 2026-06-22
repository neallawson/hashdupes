// Package verify confirms that duplicate candidates are truly identical by
// comparing their bytes. Candidate grouping (by size + head hash) only narrows
// the field; verification here is exact and is mandatory before any group is
// presented as a confirmed duplicate or acted upon destructively.
package verify

import (
	"bytes"
	"io"
	"os"
)

// compareBufSize is the read window used when streaming two files for
// comparison. Comparison short-circuits at the first differing byte.
const compareBufSize = 128 * 1024

// EqualFiles reports whether the contents of two files are byte-for-byte equal.
// It streams both files and returns as soon as a difference is found.
func EqualFiles(pathA, pathB string) (bool, error) {
	if pathA == pathB {
		return true, nil
	}
	fa, err := os.Open(pathA)
	if err != nil {
		return false, err
	}
	defer fa.Close()
	fb, err := os.Open(pathB)
	if err != nil {
		return false, err
	}
	defer fb.Close()
	return equalReaders(fa, fb)
}

func equalReaders(a, b io.Reader) (bool, error) {
	bufA := make([]byte, compareBufSize)
	bufB := make([]byte, compareBufSize)
	for {
		na, errA := io.ReadFull(a, bufA)
		nb, errB := io.ReadFull(b, bufB)
		if na != nb || !bytes.Equal(bufA[:na], bufB[:nb]) {
			return false, nil
		}
		// Both streams ended together with equal content.
		if isEOF(errA) && isEOF(errB) {
			return true, nil
		}
		if err := readErr(errA); err != nil {
			return false, err
		}
		if err := readErr(errB); err != nil {
			return false, err
		}
	}
}

// isEOF reports whether err signals end of stream for io.ReadFull.
func isEOF(err error) bool {
	return err == io.EOF || err == io.ErrUnexpectedEOF
}

// readErr returns err unless it is a benign end-of-stream signal.
func readErr(err error) error {
	if err == nil || isEOF(err) {
		return nil
	}
	return err
}

// PartitionByContent splits a candidate group of file paths into subgroups
// whose members are byte-for-byte identical. Paths that fail to open are
// returned in errs keyed by path; they are excluded from all subgroups so a
// failed read can never cause a file to be treated as a duplicate.
func PartitionByContent(paths []string) (groups [][]string, errs map[string]error) {
	errs = make(map[string]error)
	var reps []string // one representative path per discovered subgroup
	idxByRep := make(map[string]int)

	for _, p := range paths {
		matched := false
		for _, rep := range reps {
			eq, err := EqualFiles(rep, p)
			if err != nil {
				// Defer the decision: try other reps, else record below.
				continue
			}
			if eq {
				groups[idxByRep[rep]] = append(groups[idxByRep[rep]], p)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		// Open p once to surface any access error before starting a subgroup.
		if f, err := os.Open(p); err != nil {
			errs[p] = err
			continue
		} else {
			_ = f.Close()
		}
		groups = append(groups, []string{p})
		idxByRep[p] = len(groups) - 1
		reps = append(reps, p)
	}
	return groups, errs
}
