// Package qr is a pure-Go, CGO-free QR Code (ISO/IEC 18004) encoder.
//
// It encodes a byte payload into a module matrix that a consumer renders however
// it likes — a toolkit widget, an SVG, a bitmap. The core encoder draws nothing
// and imports nothing outside the standard library's error helpers; the optional
// PNG/image helpers (see image.go) are the only part that touches image/png, and
// they too rely on the standard library alone.
//
// Byte mode is used exclusively (the full 8-bit range, ISO-8859-1/binary), which
// is the right choice for arbitrary payloads such as URLs and base64 envelopes.
// The smallest version (1..40) that fits the data at the requested
// error-correction level is selected automatically unless a fixed version is
// requested.
package qr

import (
	"errors"
	"fmt"
)

// ECLevel is a QR Code error-correction level. Higher levels recover from more
// damage at the cost of capacity.
type ECLevel int

const (
	// Low recovers ~7% of codewords. It maximises data capacity and is the right
	// choice for large payloads that must stay scannable at a reasonable size.
	Low ECLevel = iota
	// Medium recovers ~15% of codewords. It is the default.
	Medium
	// Quartile recovers ~25% of codewords.
	Quartile
	// High recovers ~30% of codewords.
	High
)

// String returns the single-letter name of the level (L, M, Q, H).
func (l ECLevel) String() string {
	switch l {
	case Low:
		return "L"
	case Medium:
		return "M"
	case Quartile:
		return "Q"
	case High:
		return "H"
	default:
		return fmt.Sprintf("ECLevel(%d)", int(l))
	}
}

// formatBits is the 2-bit value written into the format information for each
// error-correction level (note this is not the same ordering as the constants).
func (l ECLevel) formatBits() int {
	return [4]int{Low: 1, Medium: 0, Quartile: 3, High: 2}[l]
}

// MinVersion and MaxVersion bound the QR Code version range defined by the
// specification.
const (
	MinVersion = 1
	MaxVersion = 40
)

// ErrTooLong is returned when the data does not fit at version 40 for the chosen
// error-correction level (or does not fit the fixed version, if one was set).
// Consumers can detect this to fall back to, e.g., copy-and-paste.
var ErrTooLong = errors.New("qr: data too long to encode")

// Matrix is an encoded QR Code as a square grid of dark/light modules, optionally
// surrounded by a light quiet zone. It carries no rendering logic; a consumer
// reads it with Size and Module.
type Matrix struct {
	// Version is the QR Code version (1..40).
	Version int
	// Level is the error-correction level used.
	Level ECLevel
	// Mask is the applied data-mask pattern (0..7).
	Mask int

	dim     int    // module count per side, excluding the quiet zone
	quiet   int    // quiet-zone width in modules, on every side
	modules []bool // dim*dim, row-major; true == dark
}

// Size returns the side length of the code in modules, including the quiet zone
// on both sides.
func (m *Matrix) Size() int { return m.dim + 2*m.quiet }

// Dimension returns the side length of the code in modules, excluding the quiet
// zone (i.e. 21 for version 1, up to 177 for version 40).
func (m *Matrix) Dimension() int { return m.dim }

// QuietZone returns the quiet-zone width in modules applied on every side.
func (m *Matrix) QuietZone() int { return m.quiet }

// Module reports whether the module at (x, y) is dark. Coordinates are in the
// Size coordinate space, i.e. they include the quiet zone; the origin is the
// top-left corner. Coordinates in the quiet zone or out of range are light
// (false).
func (m *Matrix) Module(x, y int) bool {
	x -= m.quiet
	y -= m.quiet
	if x < 0 || y < 0 || x >= m.dim || y >= m.dim {
		return false
	}
	return m.modules[y*m.dim+x]
}

// at/set access the module grid in dimension-local (quiet-zone-excluded)
// coordinates.
func (m *Matrix) at(x, y int) bool     { return m.modules[y*m.dim+x] }
func (m *Matrix) set(x, y int, v bool) { m.modules[y*m.dim+x] = v }

// config holds resolved encoding options.
type config struct {
	level      ECLevel
	minVersion int
	maxVersion int
	mask       int // -1 for automatic selection
	quietZone  int
}

// Option configures Encode.
type Option func(*config)

// WithLevel sets the error-correction level (default Medium).
func WithLevel(l ECLevel) Option { return func(c *config) { c.level = l } }

// WithVersion pins the QR Code version to a fixed value in 1..40, disabling
// automatic version selection. Encode returns an error if the version is out of
// range or the data does not fit it.
func WithVersion(v int) Option {
	return func(c *config) { c.minVersion, c.maxVersion = v, v }
}

// WithVersionRange constrains automatic version selection to min..max
// (inclusive). Encode picks the smallest version in the range that fits.
func WithVersionRange(min, max int) Option {
	return func(c *config) { c.minVersion, c.maxVersion = min, max }
}

// WithMask pins the data-mask pattern to a fixed value in 0..7, disabling
// automatic mask selection. Values outside 0..7 select automatically.
func WithMask(m int) Option { return func(c *config) { c.mask = m } }

// WithQuietZone sets the quiet-zone width in modules applied on every side
// (default 4, the value the specification recommends). Negative values are
// treated as 0.
func WithQuietZone(n int) Option { return func(c *config) { c.quietZone = n } }

// ByteCapacity returns the maximum number of payload bytes that fit at the given
// version (1..40) and error-correction level, in byte mode. It returns 0 for an
// out-of-range version.
func ByteCapacity(version int, level ECLevel) int {
	if version < MinVersion || version > MaxVersion {
		return 0
	}
	// Available data bits minus the mode indicator (4) and the character-count
	// indicator (8 bits for versions 1..9, 16 bits for 10..40). The smallest
	// data allowance (version 1, level H: 9 codewords) leaves 60 bits, so this
	// is always positive for a valid version.
	dataBits := numDataCodewords(version, level) * 8
	return (dataBits - 4 - charCountBits(version)) / 8
}

// charCountBits returns the width of the byte-mode character-count indicator for
// the given version.
func charCountBits(version int) int {
	if version <= 9 {
		return 8
	}
	return 16
}

// Encode encodes data in byte mode into a QR Code matrix. By default it uses
// error-correction level Medium, a quiet zone of 4 modules, automatic version
// selection (smallest fitting) and automatic mask selection (lowest penalty).
//
// It returns ErrTooLong (wrapped) if the data does not fit the selected or
// requested version at the chosen error-correction level.
func Encode(data []byte, opts ...Option) (*Matrix, error) {
	cfg := config{level: Medium, minVersion: MinVersion, maxVersion: MaxVersion, mask: -1, quietZone: 4}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.level < Low || cfg.level > High {
		return nil, fmt.Errorf("qr: invalid error-correction level %d", int(cfg.level))
	}
	if cfg.minVersion < MinVersion || cfg.maxVersion > MaxVersion || cfg.minVersion > cfg.maxVersion {
		return nil, fmt.Errorf("qr: invalid version range %d..%d", cfg.minVersion, cfg.maxVersion)
	}
	if cfg.quietZone < 0 {
		cfg.quietZone = 0
	}

	// Select the smallest version in range that fits.
	version := -1
	for v := cfg.minVersion; v <= cfg.maxVersion; v++ {
		if len(data) <= ByteCapacity(v, cfg.level) {
			version = v
			break
		}
	}
	if version < 0 {
		return nil, fmt.Errorf("%w: %d bytes at level %s exceeds version %d capacity (%d bytes)",
			ErrTooLong, len(data), cfg.level, cfg.maxVersion, ByteCapacity(cfg.maxVersion, cfg.level))
	}

	codewords := makeCodewords(data, version, cfg.level)
	allCodewords := addEccAndInterleave(codewords, version, cfg.level)

	m := &Matrix{
		Version: version,
		Level:   cfg.level,
		dim:     dimension(version),
		quiet:   cfg.quietZone,
	}
	m.modules = make([]bool, m.dim*m.dim)
	isFunction := make([]bool, m.dim*m.dim)
	m.drawFunctionPatterns(version, isFunction)
	m.drawCodewords(allCodewords, isFunction)
	m.Mask = m.applyBestMask(cfg.mask, isFunction)
	return m, nil
}

// dimension returns the module count per side for the given version.
func dimension(version int) int { return version*4 + 17 }

// makeCodewords builds the padded data-codeword sequence (before EC/interleave):
// the byte-mode header, the payload, the terminator, byte-alignment padding and
// the alternating pad bytes 0xEC/0x11.
func makeCodewords(data []byte, version int, level ECLevel) []byte {
	capacityBits := numDataCodewords(version, level) * 8
	bb := bitBuffer{}
	bb.appendBits(0b0100, 4)                         // byte-mode indicator
	bb.appendBits(len(data), charCountBits(version)) // character count
	for _, b := range data {
		bb.appendBits(int(b), 8)
	}
	// Terminator: four zero bits. In byte mode the capacity check guarantees at
	// least four bits remain (the maximum byte payload leaves exactly four), so
	// the "shorten the terminator near capacity" case never arises here.
	bb.appendBits(0, 4)
	// Pad to a byte boundary (a no-op in byte mode, where the stream is already
	// byte-aligned after the terminator, but kept for correctness).
	bb.appendBits(0, (8-bb.len()%8)%8)
	// Fill remaining capacity with the alternating pad codewords.
	for pad := 0xEC; bb.len() < capacityBits; pad ^= 0xEC ^ 0x11 {
		bb.appendBits(pad, 8)
	}
	return bb.bytes()
}

// addEccAndInterleave appends Reed–Solomon error-correction codewords to the data
// and interleaves the blocks as required by the specification, returning the full
// codeword sequence to be placed in the matrix.
func addEccAndInterleave(data []byte, version int, level ECLevel) []byte {
	numBlocks := numErrorCorrectionBlocks[level][version]
	blockEccLen := eccCodewordsPerBlock[level][version]
	rawCodewords := numRawCodewords(version)
	numShortBlocks := numBlocks - rawCodewords%numBlocks
	shortBlockLen := rawCodewords / numBlocks

	blocks := make([][]byte, numBlocks)
	rsDiv := rsComputeDivisor(blockEccLen)
	k := 0
	for i := 0; i < numBlocks; i++ {
		datLen := shortBlockLen - blockEccLen
		if i >= numShortBlocks {
			datLen++
		}
		dat := data[k : k+datLen]
		k += datLen
		block := make([]byte, shortBlockLen+1)
		copy(block, dat)
		ecc := rsComputeRemainder(dat, rsDiv)
		copy(block[len(block)-blockEccLen:], ecc)
		blocks[i] = block
	}

	result := make([]byte, 0, rawCodewords)
	for i := 0; i < len(blocks[0]); i++ {
		for j := 0; j < len(blocks); j++ {
			// Skip the padding cell that only short blocks lack.
			if i != shortBlockLen-blockEccLen || j >= numShortBlocks {
				result = append(result, blocks[j][i])
			}
		}
	}
	return result
}
