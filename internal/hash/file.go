package hash

import (
	"io"
	"os"
)

// DefaultHeadBytes is the default number of leading bytes hashed per file.
// 64 KiB keeps per-file I/O roughly constant regardless of file size while
// providing a strong bucketing key for duplicate candidates.
const DefaultHeadBytes int64 = 64 * 1024

// effectiveHeadLen returns the number of bytes to read for the head hash given
// the file size and the configured head length. A headBytes <= 0 means "hash
// the entire file".
func effectiveHeadLen(size, headBytes int64) int64 {
	if headBytes <= 0 || size < headBytes {
		return size
	}
	return headBytes
}

// HeadHash computes the head hash of r, reading at most n bytes where
// n = effectiveHeadLen(size, headBytes). The caller is responsible for passing
// the true file size so that small files are fully hashed.
func HeadHash(r io.Reader, size, headBytes int64, a Algo) ([]byte, error) {
	h, err := New(a)
	if err != nil {
		return nil, err
	}
	n := effectiveHeadLen(size, headBytes)
	if n > 0 {
		if _, err := io.CopyN(h, r, n); err != nil && err != io.EOF {
			return nil, err
		}
	}
	return h.Sum(nil), nil
}

// HeadHashFile opens path and computes its head hash. size should be the file's
// reported size (from a prior stat) so the read can be bounded precisely.
func HeadHashFile(path string, size, headBytes int64, a Algo) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return HeadHash(f, size, headBytes, a)
}
