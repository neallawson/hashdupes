// Package hash provides the content-hashing primitives for hashdupes: a
// pluggable hash algorithm, per-file "head" hashing (first N bytes), and the
// content-only folder Merkle hash.
package hash

import (
	"crypto/sha256"
	"fmt"
	"hash"

	"github.com/zeebo/blake3"
)

// Algo identifies a supported hash algorithm. The set is intentionally small
// and the default is BLAKE3 for speed.
type Algo string

const (
	AlgoBLAKE3 Algo = "blake3"
	AlgoSHA256 Algo = "sha256"
)

// DefaultAlgo is used when none is specified.
const DefaultAlgo = AlgoBLAKE3

// Valid reports whether a is a supported algorithm.
func (a Algo) Valid() bool {
	switch a {
	case AlgoBLAKE3, AlgoSHA256:
		return true
	default:
		return false
	}
}

// New returns a fresh streaming hash.Hash for the given algorithm.
func New(a Algo) (hash.Hash, error) {
	switch a {
	case AlgoBLAKE3:
		return blake3.New(), nil
	case AlgoSHA256:
		return sha256.New(), nil
	default:
		return nil, fmt.Errorf("hash: unknown algorithm %q", a)
	}
}

// MustNew is like New but panics on an invalid algorithm. Intended for call
// sites that have already validated the algorithm.
func MustNew(a Algo) hash.Hash {
	h, err := New(a)
	if err != nil {
		panic(err)
	}
	return h
}
