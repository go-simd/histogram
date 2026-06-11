package histogram

import (
	"fmt"
	"math/rand"
	"testing"
)

// reference is the obvious, trivially-correct single-table histogram used as
// the oracle for every other implementation.
func reference(data []byte) [256]uint32 {
	var c [256]uint32
	for _, b := range data {
		c[b]++
	}
	return c
}

func TestCountTable(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"emptySlice", []byte{}},
		{"single", []byte{0x42}},
		{"byte0", []byte{0, 0, 0, 0}},
		{"byte255", []byte{255, 255, 255}},
		{"allSame", bytesRepeat(0xAB, 1000)},
		{"everyByteOnce", everyByteOnce()},
		{"everyByteOnceX3", repeatSlice(everyByteOnce(), 3)},
		{"countUp", countUp(777)},
		{"tail7", bytesRepeat(9, 7)},   // shorter than one 8-byte block
		{"block8", bytesRepeat(9, 8)},  // exactly one block, no tail
		{"block8p1", bytesRepeat(9, 9)}, // one block + one tail byte
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Count(tc.data)
			want := reference(tc.data)
			if got != want {
				t.Fatalf("Count mismatch: %s", firstDiff(got, want))
			}
			// CountInto must accumulate, not overwrite.
			acc := [256]uint32{}
			CountInto(&acc, tc.data)
			CountInto(&acc, tc.data)
			for i := range acc {
				if acc[i] != 2*want[i] {
					t.Fatalf("CountInto did not accumulate at %d: got %d want %d", i, acc[i], 2*want[i])
				}
			}
		})
	}
}

func TestCountRandomSizes(t *testing.T) {
	r := rand.New(rand.NewSource(0xC0FFEE))
	for _, n := range []int{0, 1, 2, 3, 5, 7, 8, 9, 15, 16, 17, 31, 33, 63, 100, 255, 256, 1000, 4096, 65537} {
		data := make([]byte, n)
		r.Read(data)
		if got, want := Count(data), reference(data); got != want {
			t.Fatalf("n=%d: %s", n, firstDiff(got, want))
		}
	}
}

func TestCountSkewed(t *testing.T) {
	// Heavily repetitive data is the case a single-table loop handles worst and
	// the multi-table loop must still get exactly right.
	r := rand.New(rand.NewSource(7))
	data := make([]byte, 20000)
	for i := range data {
		if r.Intn(10) != 0 {
			data[i] = 0x55
		} else {
			data[i] = byte(r.Intn(256))
		}
	}
	if got, want := Count(data), reference(data); got != want {
		t.Fatalf("%s", firstDiff(got, want))
	}
}

func FuzzCount(f *testing.F) {
	f.Add([]byte(nil))
	f.Add([]byte{0})
	f.Add([]byte("the quick brown fox"))
	f.Add(bytesRepeat(0xFF, 33))
	f.Add(everyByteOnce())
	f.Fuzz(func(t *testing.T, data []byte) {
		if got, want := Count(data), reference(data); got != want {
			t.Fatalf("len=%d: %s", len(data), firstDiff(got, want))
		}
	})
}

// --- helpers ---

func bytesRepeat(b byte, n int) []byte {
	s := make([]byte, n)
	for i := range s {
		s[i] = b
	}
	return s
}

func everyByteOnce() []byte {
	s := make([]byte, 256)
	for i := range s {
		s[i] = byte(i)
	}
	return s
}

func repeatSlice(s []byte, times int) []byte {
	var out []byte
	for i := 0; i < times; i++ {
		out = append(out, s...)
	}
	return out
}

func countUp(n int) []byte {
	s := make([]byte, n)
	for i := range s {
		s[i] = byte(i)
	}
	return s
}

// firstDiff returns a short description of the first differing bin, to keep
// failure output readable (a full [256]uint32 dump is unhelpful).
func firstDiff(got, want [256]uint32) string {
	for i := range got {
		if got[i] != want[i] {
			return fmt.Sprintf("bin %d: got %d want %d", i, got[i], want[i])
		}
	}
	return "identical"
}
