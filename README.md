# histogram

[![ci](https://github.com/go-simd/histogram/actions/workflows/ci.yml/badge.svg)](https://github.com/go-simd/histogram/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-simd/histogram.svg)](https://pkg.go.dev/github.com/go-simd/histogram)

The fastest correct **byte-value histogram** in pure Go (`CGO_ENABLED=0`, stable
Go, no assembly): count how many times each of the 256 possible byte values
occurs in a `[]byte`.

```go
import "github.com/go-simd/histogram"

c := histogram.Count(data) // c[v] == number of bytes in data equal to v

var acc [256]uint32
histogram.CountInto(&acc, a) // accumulate several slices into one table
histogram.CountInto(&acc, b)
```

`Count` returns a fresh `[256]uint32` (no heap allocation). `CountInto` adds into
a caller-supplied array, so you can fold multiple buffers into one histogram.

## Why there is no SIMD kernel (the honest answer)

A byte histogram is a **scatter-add**: `c[data[i]]++`. Each input byte indexes a
*different, data-dependent* counter and read-modify-writes it. That is precisely
the access pattern SIMD does **not** have:

* AVX2 and ARM NEON have **gather but no scatter**. You cannot vectorise
  `c[idx]++` across a lane vector because two lanes may target the same counter
  (a collision) and the writes would race within the instruction.
* Only **AVX-512** has a true vector scatter — and the dev/CI hardware here is
  Zen3-class with **no AVX-512**. Even where scatter exists, colliding bins
  serialise, so it rarely beats a good scalar loop on real (skewed) data.

So the win does **not** come from SIMD. It comes from **instruction-level
parallelism**: keep several independent `[256]uint32` partial tables and
round-robin consecutive bytes across them, then sum the tables once at the end.
Spreading neighbouring bytes across distinct memory cells **breaks the
store-to-load dependency** that stalls a single-table loop whenever a byte value
repeats — the classic "counting bytes fast" trick from FSE
([fastcompression.blogspot.com](http://fastcompression.blogspot.com/2014/09/counting-bytes-fast-little-trick-from.html)).

`Count` uses **eight partial tables with word-batched loads**. This was the
fastest of every variant measured (single, 2/4/8 tables, with and without
word-batching): eight tables fully hide the increment latency even on
adversarial low-entropy input, where a single-table loop collapses.

## Performance

Throughput on a 1 MiB buffer, `go test -bench . -count=6`, best-of run.
Two input distributions: **uniform** random bytes, and **skew90** (90% one
value — the low-entropy case that wrecks a naive single-table loop).

### Native amd64 (authoritative — GitHub Actions `ubuntu-latest`)

| approach                         | uniform GB/s | skew90 GB/s |
| -------------------------------- | -----------: | ----------: |
| single table (naive baseline)    |   _see CI_    |   _see CI_   |
| 4 partial tables                 |   _see CI_    |   _see CI_   |
| **8 word-batched tables (`Count`)** | _see CI_   |   _see CI_   |

> The amd64 row is filled from the `bench` workflow. The dev machine is arm64,
> so amd64 numbers must come from native CI, not from Rosetta (which has no
> AVX2 — irrelevant here since we ship no asm, but the timing is still
> unrepresentative under emulation).

### arm64 (indicative — Apple M-series dev box)

| approach                         | uniform GB/s | skew90 GB/s |
| -------------------------------- | -----------: | ----------: |
| single table (naive baseline)    |         3.73 |        0.58 |
| 4 partial tables                 |         3.77 |        2.28 |
| **8 word-batched tables (`Count`)** |      4.52 |        2.83 |

**The finding:** SIMD does not help a byte histogram on AVX2/NEON hardware — it
is a scatter, and there is no scatter instruction. The multi-table scalar loop
is the real winner. On uniform data it edges out the single-table baseline; on
skewed data it is **~5× faster**, because the single-table loop serialises on
the repeated counter while the eight tables keep the pipeline full.

## Correctness

`Count` is validated against a trivial single-table reference oracle by a table
test (empty, single byte, all-same, every-byte-once, block-boundary sizes),
random-size property tests, a skewed-data test, and a `FuzzCount` differential
fuzz target (run for 15 s in CI on both amd64 and arm64).

## Existing work

There was no pure-Go library exposing a fast `Count(data) [256]uint32` API.
[`vteromero/byte-hist`](https://github.com/vteromero/byte-hist) is a CLI tool,
not a reusable package, and `valyala/histogram` is a float-quantile sketch,
unrelated to byte frequency counting. This repo fills that gap.

## License

BSD 3-Clause. See [LICENSE](LICENSE).
