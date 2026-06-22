package hash

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

func TestNewAlgorithms(t *testing.T) {
	for _, a := range []Algo{AlgoBLAKE3, AlgoSHA256} {
		if _, err := New(a); err != nil {
			t.Fatalf("New(%q) error: %v", a, err)
		}
	}
	if _, err := New(Algo("bogus")); err == nil {
		t.Fatal("expected error for unknown algorithm")
	}
}

func TestHeadHashFullVsTruncated(t *testing.T) {
	data := bytes.Repeat([]byte("abcd"), 1000) // 4000 bytes
	size := int64(len(data))

	// headBytes >= size hashes the whole file.
	full, err := HeadHash(bytes.NewReader(data), size, size, AlgoSHA256)
	if err != nil {
		t.Fatal(err)
	}
	whole, err := HeadHash(bytes.NewReader(data), size, 0, AlgoSHA256) // 0 => whole
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(full, whole) {
		t.Fatal("hashing all bytes via headBytes==size should equal whole-file hash")
	}

	// A smaller head should only depend on the leading bytes.
	head1, err := HeadHash(bytes.NewReader(data), size, 16, AlgoSHA256)
	if err != nil {
		t.Fatal(err)
	}
	// Different tail, identical first 16 bytes => identical head hash.
	mutated := append(append([]byte{}, data[:16]...), bytes.Repeat([]byte("ZZZZ"), 996)...)
	head2, err := HeadHash(bytes.NewReader(mutated), int64(len(mutated)), 16, AlgoSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(head1, head2) {
		t.Fatal("head hash should depend only on the leading headBytes")
	}
}

func TestFolderHashOrderIndependence(t *testing.T) {
	hA := sha("a")
	hB := sha("b")
	c1 := []Child{{Hash: hA, Size: 1}, {Hash: hB, Size: 2}}
	c2 := []Child{{Hash: hB, Size: 2}, {Hash: hA, Size: 1}}

	g1, err := FolderHash(c1, AlgoSHA256)
	if err != nil {
		t.Fatal(err)
	}
	g2, err := FolderHash(c2, AlgoSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(g1, g2) {
		t.Fatal("folder hash must be independent of child ordering")
	}
}

func TestFolderHashSizeMatters(t *testing.T) {
	h := sha("same-head")
	a, _ := FolderHash([]Child{{Hash: h, Size: 10}}, AlgoSHA256)
	b, _ := FolderHash([]Child{{Hash: h, Size: 20}}, AlgoSHA256)
	if bytes.Equal(a, b) {
		t.Fatal("file children with identical head hash but different size must differ")
	}
}

func TestEmptyFolderHashStable(t *testing.T) {
	a, _ := EmptyFolderHash(AlgoSHA256)
	b, _ := FolderHash(nil, AlgoSHA256)
	if !bytes.Equal(a, b) {
		t.Fatal("EmptyFolderHash must equal FolderHash(nil)")
	}
}

// sha is a tiny helper producing a deterministic 32-byte hash for test inputs.
func sha(s string) []byte {
	h := MustNew(AlgoSHA256)
	_, _ = h.Write([]byte(s))
	return h.Sum(nil)
}

// guard against accidental hex/length assumptions in tokens.
func TestTokenFormat(t *testing.T) {
	c := Child{Hash: sha("x"), Size: 42}
	tok := c.token()
	if !strings.HasPrefix(tok, "42:") {
		t.Fatalf("file token should start with size: got %q", tok)
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(tok, "42:")); err != nil {
		t.Fatalf("token hash segment should be hex: %v", err)
	}
}
