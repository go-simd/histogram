package histogram

import (
	"math/rand"
	"testing"
)

// The benchmarks below pit the exported Count (eight word-batched partial
// tables) against the candidate approaches it was selected over: a single-table
// scalar loop and four-/eight-table variants. They run over both uniform and
// skewed (90% one value) input, since the single-table loop only collapses on
// skew. SetBytes reports throughput as MB/s; divide by 1000 for GB/s.

const benchSize = 1 << 20 // 1 MiB

func benchData(skew bool) []byte {
	r := rand.New(rand.NewSource(1))
	b := make([]byte, benchSize)
	if !skew {
		r.Read(b)
		return b
	}
	for i := range b {
		if r.Intn(10) != 0 {
			b[i] = 0
		} else {
			b[i] = byte(r.Intn(256))
		}
	}
	return b
}

// single is the naive one-table baseline (identical to reference, kept separate
// so the benchmark name is unambiguous).
func single(data []byte) [256]uint32 {
	var c [256]uint32
	for _, v := range data {
		c[v]++
	}
	return c
}

func multi4(data []byte) [256]uint32 {
	var c0, c1, c2, c3 [256]uint32
	n := len(data)
	i := 0
	for ; i+4 <= n; i += 4 {
		c0[data[i]]++
		c1[data[i+1]]++
		c2[data[i+2]]++
		c3[data[i+3]]++
	}
	for ; i < n; i++ {
		c0[data[i]]++
	}
	var c [256]uint32
	for j := 0; j < 256; j++ {
		c[j] = c0[j] + c1[j] + c2[j] + c3[j]
	}
	return c
}

func multi4word(data []byte) [256]uint32 {
	var c0, c1, c2, c3 [256]uint32
	n := len(data)
	i := 0
	for ; i+8 <= n; i += 8 {
		b0, b1, b2, b3 := data[i], data[i+1], data[i+2], data[i+3]
		b4, b5, b6, b7 := data[i+4], data[i+5], data[i+6], data[i+7]
		c0[b0]++
		c1[b1]++
		c2[b2]++
		c3[b3]++
		c0[b4]++
		c1[b5]++
		c2[b6]++
		c3[b7]++
	}
	for ; i < n; i++ {
		c0[data[i]]++
	}
	var c [256]uint32
	for j := 0; j < 256; j++ {
		c[j] = c0[j] + c1[j] + c2[j] + c3[j]
	}
	return c
}

func multi8(data []byte) [256]uint32 {
	var c0, c1, c2, c3, c4, c5, c6, c7 [256]uint32
	n := len(data)
	i := 0
	for ; i+8 <= n; i += 8 {
		c0[data[i]]++
		c1[data[i+1]]++
		c2[data[i+2]]++
		c3[data[i+3]]++
		c4[data[i+4]]++
		c5[data[i+5]]++
		c6[data[i+6]]++
		c7[data[i+7]]++
	}
	for ; i < n; i++ {
		c0[data[i]]++
	}
	var c [256]uint32
	for j := 0; j < 256; j++ {
		c[j] = c0[j] + c1[j] + c2[j] + c3[j] + c4[j] + c5[j] + c6[j] + c7[j]
	}
	return c
}

func runBench(b *testing.B, f func([]byte) [256]uint32) {
	for _, skew := range []bool{false, true} {
		name := "uniform"
		if skew {
			name = "skew90"
		}
		data := benchData(skew)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sink = f(data)
			}
		})
	}
}

var sink [256]uint32

func BenchmarkSingle(b *testing.B)     { runBench(b, single) }
func BenchmarkMulti4(b *testing.B)     { runBench(b, multi4) }
func BenchmarkMulti4Word(b *testing.B) { runBench(b, multi4word) }
func BenchmarkMulti8(b *testing.B)     { runBench(b, multi8) }
func BenchmarkCount(b *testing.B)      { runBench(b, Count) } // exported impl
