package object

import (
	"hash/fnv"
	"testing"
)

func TestFNV64aMatchesHashFNV(t *testing.T) {
	for _, s := range []string{"", "Q", "IN", "running", "__class__", "Ünïcode", "a#b"} {
		h := fnv.New64a()
		h.Write([]byte(s))
		if got, want := fnv64a(s), h.Sum64(); got != want {
			t.Errorf("fnv64a(%q) = %x, want %x", s, got, want)
		}
	}
}
