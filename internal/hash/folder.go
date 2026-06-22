package hash

import (
	"encoding/hex"
	"sort"
	"strconv"
)

// childSep separates child tokens within a folder's pre-image. It is a byte
// that cannot appear in a hex digest or decimal size, avoiding ambiguity.
const childSep = '\n'

// Child is one entry contributing to a folder's content-only Merkle hash.
//
// For a file child, Size participates in the token (as "size:hexhash") so that
// the head-only hashing scheme cannot conflate different-sized files. For a
// folder child, only its folder hash is used (its size is already folded in
// recursively).
type Child struct {
	Hash  []byte
	Size  int64
	IsDir bool
}

// token renders a child into its canonical, content-only string form.
func (c Child) token() string {
	hexHash := hex.EncodeToString(c.Hash)
	if c.IsDir {
		return hexHash
	}
	return strconv.FormatInt(c.Size, 10) + ":" + hexHash
}

// FolderHash computes the content-only Merkle hash of a folder from its
// children. Children are tokenized and sorted so the result is independent of
// directory ordering and of filenames. An empty child set yields the canonical
// empty-folder hash (see EmptyFolderHash).
func FolderHash(children []Child, a Algo) ([]byte, error) {
	h, err := New(a)
	if err != nil {
		return nil, err
	}
	tokens := make([]string, len(children))
	for i, c := range children {
		tokens[i] = c.token()
	}
	sort.Strings(tokens)
	for _, t := range tokens {
		if _, err := h.Write([]byte(t)); err != nil {
			return nil, err
		}
		if _, err := h.Write([]byte{childSep}); err != nil {
			return nil, err
		}
	}
	return h.Sum(nil), nil
}

// EmptyFolderHash returns the canonical hash for a folder with no (non-skipped)
// children. All empty folders share this value.
func EmptyFolderHash(a Algo) ([]byte, error) {
	return FolderHash(nil, a)
}
