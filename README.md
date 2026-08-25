# qr — pure-Go QR Code encoder

[![ci](https://github.com/go-gfx/qr/actions/workflows/ci.yml/badge.svg)](https://github.com/go-gfx/qr/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-gfx/qr.svg)](https://pkg.go.dev/github.com/go-gfx/qr)
[![License: BSD-3-Clause](https://img.shields.io/badge/License-BSD--3--Clause-blue.svg)](LICENSE)

`github.com/go-gfx/qr` is a pure-Go, **CGO=0** QR Code (ISO/IEC 18004) encoder. It
turns a byte payload into a **module matrix** that the caller renders however it
likes — a toolkit widget, an SVG, a bitmap. It draws no UI of its own.

It sits in the [go-gfx](https://github.com/go-gfx) 2D-graphics foundation. The
core encoder imports **nothing** outside the standard library; an optional helper
(`Image`/`PNG`) rasterises the matrix using only `image`/`image/png`.

## Why a matrix, not a picture

Consumers own rendering. The playground's "scan to connect" feature paints the
matrix with a toolkit widget; a server might emit SVG; a test might diff pixels.
`Matrix` exposes just what a renderer needs:

```go
type Matrix struct {
	Version int     // 1..40
	Level   ECLevel // L, M, Q, H
	Mask    int     // 0..7, the applied data mask
	// ...
}

func (m *Matrix) Size() int              // side length in modules, quiet zone included
func (m *Matrix) Dimension() int         // side length excluding the quiet zone
func (m *Matrix) QuietZone() int         // quiet-zone width per side
func (m *Matrix) Module(x, y int) bool   // true == dark; quiet zone / out of range == light
```

## Usage

```go
import "github.com/go-gfx/qr"

m, err := qr.Encode([]byte("https://go-gfx.dev"))
if err != nil {
	// qr.ErrTooLong (wrapped) means the payload exceeds version-40 capacity —
	// fall back to copy-and-paste.
}

for y := 0; y < m.Size(); y++ {
	for x := 0; x < m.Size(); x++ {
		if m.Module(x, y) {
			// paint a dark module at (x, y)
		}
	}
}
```

### Options

```go
m, err := qr.Encode(payload,
	qr.WithLevel(qr.Low),      // L maximises capacity (default is Medium)
	qr.WithQuietZone(4),       // modules of light border per side (default 4)
	qr.WithVersion(10),        // pin the version (default: smallest that fits)
	qr.WithVersionRange(5, 20),// or constrain automatic selection
	qr.WithMask(3),            // pin the mask (default: lowest-penalty of all 8)
)
```

### Optional PNG / image.Image helper

```go
img := m.Image(8, 4)            // 8 px per module, +4-module quiet zone; image.Image
png, err := m.PNG(8, 4)         // the same, PNG-encoded
```

These are the only APIs that touch `image`/`image/png`; importing the package for
the matrix alone pulls in nothing beyond the standard library.

## Capacity

Byte-mode capacity, in payload bytes, at the largest versions (query any with
`qr.ByteCapacity(version, level)`):

| Version | Modules | L | M | Q | H |
|--------:|:-------:|----:|----:|----:|----:|
| 20 | 97×97 | 858 | 666 | 482 | 382 |
| 30 | 137×137 | 1732 | 1370 | 982 | 742 |
| 40 | 177×177 | **2953** | 2331 | 1663 | 1273 |

Large payloads pay for themselves in density: a version-40 symbol is 177×177
modules. Choosing `WithLevel(qr.Low)` maximises capacity (~7% recovery); a
compressed ~1–2 KB payload (e.g. a base64 SDP envelope) fits comfortably below
version 40 at L, but the caller should compress and/or cap payload size so the
symbol stays scannable at its rendered pixel size.

## Correctness

The encoder implements the full byte-mode pipeline itself: mode/character-count
header, Reed–Solomon error correction over GF(256), block interleaving, module
placement, all eight data masks with standard penalty scoring, and format/version
information.

Correctness is proven by **decoding, not just by running**: the `conformance`
module (a test-only nested module) encodes a documented corpus — short URLs,
UTF-8 text, full-range binary, a ~1.2 KB base64 blob — across every
error-correction level and a spread of versions up to 40, renders each to a
bitmap, and decodes it back with the independent pure-Go reader
[`gozxing`](https://github.com/makiuchi-d/gozxing), asserting the bytes
round-trip. That reader is isolated in the nested module, so this library's own
`go.mod` stays dependency-free.

Every statement, including error branches, is covered (`go test` gate at 100%),
the suite runs under `-race`, and the package cross-compiles CGO-free for
`amd64`, `arm64`, `riscv64`, `loong64`, `ppc64le`, `s390x` and `js/wasm`.

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-gfx authors.
