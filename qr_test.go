package qr

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"io"
	"strings"
	"testing"
)

// goldenV2M0 is the SHA-256 of the ASCII module dump produced by TestGoldenStable.
// Pinned from this encoder's output, itself validated by the gozxing round-trip
// in the conformance module.
const goldenV2M0 = "0a91beac944ce5f76cd994398e0bf71ac18867ab22b65df2b5f5cc35e42f88ba"

func TestECLevelString(t *testing.T) {
	cases := map[ECLevel]string{Low: "L", Medium: "M", Quartile: "Q", High: "H", ECLevel(99): "ECLevel(99)"}
	for lvl, want := range cases {
		if got := lvl.String(); got != want {
			t.Errorf("ECLevel(%d).String() = %q, want %q", int(lvl), got, want)
		}
	}
}

func TestFormatBits(t *testing.T) {
	want := map[ECLevel]int{Low: 1, Medium: 0, Quartile: 3, High: 2}
	for lvl, w := range want {
		if got := lvl.formatBits(); got != w {
			t.Errorf("%s.formatBits() = %d, want %d", lvl, got, w)
		}
	}
}

func TestByteCapacity(t *testing.T) {
	// Values from ISO/IEC 18004 Table 7 (byte-mode data capacity).
	cases := []struct {
		version int
		level   ECLevel
		want    int
	}{
		{1, Low, 17}, {1, Medium, 14}, {1, Quartile, 11}, {1, High, 7},
		{10, Low, 271}, {10, High, 119},
		{40, Low, 2953}, {40, Medium, 2331}, {40, Quartile, 1663}, {40, High, 1273},
		{0, Low, 0}, {41, Low, 0}, {-1, Medium, 0},
	}
	for _, c := range cases {
		if got := ByteCapacity(c.version, c.level); got != c.want {
			t.Errorf("ByteCapacity(%d, %s) = %d, want %d", c.version, c.level, got, c.want)
		}
	}
}

func TestCharCountBits(t *testing.T) {
	if charCountBits(9) != 8 || charCountBits(10) != 16 {
		t.Fatal("charCountBits boundary wrong")
	}
}

func TestEncodeBasic(t *testing.T) {
	m, err := Encode([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 1 || m.Level != Medium {
		t.Fatalf("got v%d level %s", m.Version, m.Level)
	}
	if m.Dimension() != 21 {
		t.Fatalf("dimension = %d, want 21", m.Dimension())
	}
	if m.QuietZone() != 4 {
		t.Fatalf("quiet = %d, want 4", m.QuietZone())
	}
	if m.Size() != 21+8 {
		t.Fatalf("size = %d, want 29", m.Size())
	}
}

func TestEncodeAutoVersionSelection(t *testing.T) {
	// 17 bytes fits version 1 at L; 18 needs version 2.
	m1, err := Encode(bytes.Repeat([]byte("a"), 17), WithLevel(Low))
	if err != nil {
		t.Fatal(err)
	}
	if m1.Version != 1 {
		t.Fatalf("17 bytes/L: version %d, want 1", m1.Version)
	}
	m2, err := Encode(bytes.Repeat([]byte("a"), 18), WithLevel(Low))
	if err != nil {
		t.Fatal(err)
	}
	if m2.Version != 2 {
		t.Fatalf("18 bytes/L: version %d, want 2", m2.Version)
	}
}

func TestEncodeExactCapacityNoPadding(t *testing.T) {
	// Fill version 1 / M exactly (14 bytes) to exercise the no-padding path.
	m, err := Encode(bytes.Repeat([]byte("Z"), ByteCapacity(1, Medium)), WithVersion(1))
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 1 {
		t.Fatalf("version %d", m.Version)
	}
}

func TestEncodeEmpty(t *testing.T) {
	if _, err := Encode(nil); err != nil {
		t.Fatal(err)
	}
}

func TestEncodeLargeMultiBlock(t *testing.T) {
	// A large version forces multiple EC blocks (short + long) and interleaving.
	data := bytes.Repeat([]byte("go-gfx/qr "), 250)
	m, err := Encode(data, WithLevel(Low))
	if err != nil {
		t.Fatal(err)
	}
	if m.Version < 10 {
		t.Fatalf("expected a large version, got %d", m.Version)
	}
}

func TestEncodeFixedVersionAndMask(t *testing.T) {
	for mask := 0; mask < 8; mask++ {
		m, err := Encode([]byte("fixed"), WithVersion(7), WithMask(mask))
		if err != nil {
			t.Fatalf("mask %d: %v", mask, err)
		}
		if m.Version != 7 || m.Mask != mask {
			t.Fatalf("got v%d mask %d", m.Version, m.Mask)
		}
	}
}

func TestEncodeVersionRange(t *testing.T) {
	m, err := Encode([]byte("x"), WithVersionRange(5, 10))
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 5 {
		t.Fatalf("version %d, want 5 (smallest in range)", m.Version)
	}
}

func TestEncodeQuietZoneClamp(t *testing.T) {
	m, err := Encode([]byte("q"), WithQuietZone(-5))
	if err != nil {
		t.Fatal(err)
	}
	if m.QuietZone() != 0 {
		t.Fatalf("quiet = %d, want 0", m.QuietZone())
	}
	if m.Size() != m.Dimension() {
		t.Fatal("size should equal dimension with zero quiet zone")
	}
	// Out-of-range module reads are light.
	if m.Module(-1, -1) || m.Module(m.Size()+5, 0) {
		t.Fatal("out-of-range module should be light")
	}
}

func TestEncodeErrors(t *testing.T) {
	cases := []struct {
		name string
		opts []Option
		data []byte
	}{
		{"bad level low", []Option{WithLevel(-1)}, []byte("x")},
		{"bad level high", []Option{WithLevel(99)}, []byte("x")},
		{"version 0", []Option{WithVersion(0)}, []byte("x")},
		{"version 41", []Option{WithVersion(41)}, []byte("x")},
		{"inverted range", []Option{WithVersionRange(10, 3)}, []byte("x")},
	}
	for _, c := range cases {
		if _, err := Encode(c.data, c.opts...); err == nil {
			t.Errorf("%s: expected error", c.name)
		}
	}
}

func TestEncodeTooLong(t *testing.T) {
	// Exceed version-40 L capacity.
	_, err := Encode(bytes.Repeat([]byte{0}, ByteCapacity(40, Low)+1), WithLevel(Low))
	if !errors.Is(err, ErrTooLong) {
		t.Fatalf("expected ErrTooLong, got %v", err)
	}
	// Exceed a pinned small version.
	_, err = Encode(bytes.Repeat([]byte{0}, ByteCapacity(1, High)+1), WithVersion(1), WithLevel(High))
	if !errors.Is(err, ErrTooLong) {
		t.Fatalf("expected ErrTooLong for fixed version, got %v", err)
	}
}

func TestVersionInfoDrawnForV7Plus(t *testing.T) {
	// Versions 1..6 carry no version information; 7+ do. Exercise both branches.
	if _, err := Encode([]byte("small"), WithVersion(6)); err != nil {
		t.Fatal(err)
	}
	if _, err := Encode([]byte("large"), WithVersion(7)); err != nil {
		t.Fatal(err)
	}
}

func TestAllModuleCoordinatesReadable(t *testing.T) {
	m, err := Encode([]byte("scan"), WithVersion(2))
	if err != nil {
		t.Fatal(err)
	}
	// Reading every coordinate (quiet zone included) must not panic and the
	// three finder centres must be dark.
	for y := 0; y < m.Size(); y++ {
		for x := 0; x < m.Size(); x++ {
			_ = m.Module(x, y)
		}
	}
	q := m.QuietZone()
	// Finder centre near top-left is at dimension coord (3,3).
	if !m.Module(q+3, q+3) {
		t.Fatal("top-left finder centre should be dark")
	}
}

func TestAlignmentPatternPositions(t *testing.T) {
	if alignmentPatternPositions(1) != nil {
		t.Fatal("version 1 has no alignment patterns")
	}
	// Version 2: single alignment pattern at (18,18).
	if got := alignmentPatternPositions(2); len(got) != 2 || got[0] != 6 || got[1] != 18 {
		t.Fatalf("v2 positions = %v", got)
	}
	// Version 32 takes the special step==26 branch.
	got := alignmentPatternPositions(32)
	if len(got) == 0 || got[0] != 6 {
		t.Fatalf("v32 positions = %v", got)
	}
	// Version 7 (>=7 raw-module branch) round-trips through Encode elsewhere.
	if len(alignmentPatternPositions(7)) == 0 {
		t.Fatal("v7 should have alignment patterns")
	}
}

func TestNumRawDataModules(t *testing.T) {
	// Spot-check against known raw-codeword counts.
	cases := map[int]int{1: 26, 2: 44, 7: 196, 40: 3706}
	for v, want := range cases {
		if got := numRawCodewords(v); got != want {
			t.Errorf("numRawCodewords(%d) = %d, want %d", v, got, want)
		}
	}
}

func TestReedSolomonKnownVector(t *testing.T) {
	// Version 1 / M example from the QR tutorial: the 16 data codewords below
	// produce these 10 EC codewords.
	data := []byte{32, 91, 11, 120, 209, 114, 220, 77, 67, 64, 236, 17, 236, 17, 236, 17}
	div := rsComputeDivisor(10)
	got := rsComputeRemainder(data, div)
	want := []byte{196, 35, 39, 119, 235, 215, 231, 226, 93, 23}
	if !bytes.Equal(got, want) {
		t.Fatalf("RS ecc = %v, want %v", got, want)
	}
}

func TestImageClampAndRender(t *testing.T) {
	m, err := Encode([]byte("img"), WithVersion(1), WithQuietZone(0))
	if err != nil {
		t.Fatal(err)
	}
	img := m.Image(0, -1) // scale and quietZone both clamped
	b := img.Bounds()
	if b.Dx() != m.Size() || b.Dy() != m.Size() {
		t.Fatalf("clamped image size = %v, want %d", b.Size(), m.Size())
	}
	// A dark module renders as a black pixel; check the finder centre.
	pal, ok := img.(*image.Paletted)
	if !ok {
		t.Fatal("expected paletted image")
	}
	if pal.ColorIndexAt(3, 3) != 1 {
		t.Fatal("finder centre pixel should be black")
	}
	// Scale up and add a quiet zone.
	big := m.Image(4, 2)
	if big.Bounds().Dx() != (m.Size()+4)*4 {
		t.Fatalf("scaled size = %d", big.Bounds().Dx())
	}
}

func TestPNG(t *testing.T) {
	m, err := Encode([]byte("png"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := m.PNG(4, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("output is not a PNG")
	}
}

func TestPNGEncodeError(t *testing.T) {
	// Force the encoder seam to fail to cover the error branch.
	orig := pngEncode
	defer func() { pngEncode = orig }()
	sentinel := errors.New("boom")
	pngEncode = func(w io.Writer, img image.Image) error { return sentinel }
	m, _ := Encode([]byte("x"))
	if _, err := m.PNG(1, 1); !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}

// TestGoldenStable pins the module bitmap of a fixed input so an accidental
// regression in placement/masking is caught even without the reference decoder.
// The value was produced by this encoder and validated by the gozxing
// round-trip in the conformance module.
func TestGoldenStable(t *testing.T) {
	m, err := Encode([]byte("https://go-gfx.dev/qr"), WithVersion(2), WithLevel(Medium), WithMask(0), WithQuietZone(0))
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for y := 0; y < m.Dimension(); y++ {
		for x := 0; x < m.Dimension(); x++ {
			if m.Module(x, y) {
				sb.WriteByte('#')
			} else {
				sb.WriteByte(' ')
			}
		}
		sb.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(sb.String()))
	got := hex.EncodeToString(sum[:])
	const want = goldenV2M0
	if got != want {
		t.Fatalf("golden hash changed:\n got  %s\n want %s\n(if this change is intentional and the conformance decode still passes, update goldenV2M0)", got, want)
	}
}
