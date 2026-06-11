// Package histogram counts how many times each of the 256 possible byte values
// occurs in a []byte — a byte-value histogram (a.k.a. frequency table), the
// building block of entropy coders (FSE/Huffman/rANS), data profiling and
// compression heuristics.
//
// A byte histogram is a scatter-add: c[data[i]]++. That access pattern does not
// map onto AVX2/NEON, which have gather but no scatter (only AVX-512 has a true
// vector scatter, and even there contended bins serialise). The fast, portable
// technique is therefore not SIMD but instruction-level parallelism: keep
// several independent [256]uint32 tables and round-robin consecutive bytes
// across them, then sum the tables once at the end. Spreading neighbouring
// bytes over distinct memory cells breaks the store-to-load dependency that
// otherwise stalls a single-table loop whenever a value repeats — exactly the
// case (skewed/low-entropy data) where a naive histogram collapses.
//
// Count uses eight partial tables with word-batched loads. See the package
// README for the measured throughput and the reasoning behind choosing this
// over both a single-table scalar loop and a SIMD attempt.
package histogram

// Count returns the byte-value histogram of data: the returned array's element
// i is the number of bytes in data equal to i. It allocates nothing on the heap
// (the result is returned by value) and is safe for concurrent use on distinct
// slices.
func Count(data []byte) [256]uint32 {
	var c [256]uint32
	CountInto(&c, data)
	return c
}

// CountInto adds the byte-value histogram of data into *c, leaving the existing
// counts in *c untouched (it accumulates). Pass a zeroed array for a fresh
// histogram, or call it repeatedly to fold several slices into one table. c must
// be non-nil.
//
// It keeps eight independent partial tables so that runs of equal bytes land in
// eight different counters, breaking the store-to-load dependency chain that
// stalls a single-table loop on repetitive input; the partials are summed back
// into *c at the end.
func CountInto(c *[256]uint32, data []byte) {
	var c0, c1, c2, c3, c4, c5, c6, c7 [256]uint32
	n := len(data)
	i := 0
	// Eight bytes per iteration, one per table. The explicit locals let the
	// compiler issue all eight loads before the dependent increments, and the
	// up-front length check elides the per-access bounds checks.
	for ; i+8 <= n; i += 8 {
		b0 := data[i]
		b1 := data[i+1]
		b2 := data[i+2]
		b3 := data[i+3]
		b4 := data[i+4]
		b5 := data[i+5]
		b6 := data[i+6]
		b7 := data[i+7]
		c0[b0]++
		c1[b1]++
		c2[b2]++
		c3[b3]++
		c4[b4]++
		c5[b5]++
		c6[b6]++
		c7[b7]++
	}
	// Tail (fewer than 8 bytes left) into the first table.
	for ; i < n; i++ {
		c0[data[i]]++
	}
	for j := 0; j < 256; j++ {
		c[j] += c0[j] + c1[j] + c2[j] + c3[j] + c4[j] + c5[j] + c6[j] + c7[j]
	}
}
