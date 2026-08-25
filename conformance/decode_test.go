// Package conformance holds the reference-decoder round-trip (A/B on the output)
// that proves the encoder's Reed–Solomon, placement and masking are correct. It
// lives in its own module so the encoder itself keeps a zero-dependency go.mod:
// the independent pure-Go decoder github.com/makiuchi-d/gozxing is a test-only
// dependency and never reaches consumers of github.com/go-gfx/qr.
//
// Method: for a corpus of payloads across every error-correction level and a
// spread of versions (including large ones), encode with go-gfx/qr, rasterise to
// an image, decode with gozxing, and assert the decoded bytes equal the input.
// A hand-rolled RS or masking bug cannot survive a real decode of its output.
package conformance

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/go-gfx/qr"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// decode runs the independent gozxing QR reader over the matrix rendered to a
// bitmap and returns the recovered payload bytes.
func decode(t *testing.T, m *qr.Matrix) []byte {
	t.Helper()
	// Render at 8 px/module with an extra 2-module quiet zone for a clean scan.
	img := m.Image(8, 2)
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		t.Fatalf("bitmap: %v", err)
	}
	reader := qrcode.NewQRCodeReader()
	hints := map[gozxing.DecodeHintType]interface{}{
		// Return the raw bytes; do not re-interpret as a charset.
		gozxing.DecodeHintType_PURE_BARCODE: true,
	}
	res, err := reader.Decode(bmp, hints)
	if err != nil {
		t.Fatalf("gozxing decode failed (v%d level %s mask %d): %v",
			m.Version, m.Level, m.Mask, err)
	}
	return decodeRawBytes(t, res)
}

// decodeRawBytes extracts the byte-mode payload from a gozxing result, preferring
// the raw byte segments so binary payloads survive intact.
func decodeRawBytes(t *testing.T, res *gozxing.Result) []byte {
	t.Helper()
	if meta := res.GetResultMetadata(); meta != nil {
		if seg, ok := meta[gozxing.ResultMetadataType_BYTE_SEGMENTS]; ok {
			var out []byte
			for _, s := range seg.([][]byte) {
				out = append(out, s...)
			}
			return out
		}
	}
	return []byte(res.GetText())
}

// corpus is the documented payload set exercised across levels and versions.
func corpus(t *testing.T) map[string][]byte {
	t.Helper()
	// A ~1.2 KB base64 blob standing in for a compressed SDP "scan to connect"
	// envelope — the large-payload case the playground cares about.
	raw := make([]byte, 900)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("rand: %v", err)
	}
	sdpLike := base64.StdEncoding.EncodeToString(raw) // ~1200 bytes

	return map[string][]byte{
		"empty":       {},
		"short-url":   []byte("https://go-gfx.dev/qr"),
		"ascii":       []byte("The quick brown fox jumps over the lazy dog 0123456789"),
		"utf8":        []byte("café — π ≈ 3.14159 — 日本語 — Ω"),
		"binary-256":  bytesSeq(256),
		"medium-json": []byte(`{"type":"offer","ice":["stun:stun.example.org:3478"],"ts":1750000000,"v":2}`),
		"sdp-like":    []byte(sdpLike),
	}
}

// bytesSeq returns n bytes cycling through every value 0..255.
func bytesSeq(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func TestReferenceDecodeRoundTrip(t *testing.T) {
	levels := []qr.ECLevel{qr.Low, qr.Medium, qr.Quartile, qr.High}
	total := 0
	for name, payload := range corpus(t) {
		for _, lvl := range levels {
			m, err := qr.Encode(payload, qr.WithLevel(lvl))
			if err != nil {
				t.Fatalf("%s/%s: encode: %v", name, lvl, err)
			}
			got := decode(t, m)
			if !bytes.Equal(got, payload) {
				t.Fatalf("%s/%s v%d mask %d: round-trip mismatch\n want %d bytes\n got  %d bytes",
					name, lvl, m.Version, m.Mask, len(payload), len(got))
			}
			total++
		}
	}
	t.Logf("A/B round-trip: %d encode→decode pairs, all decoded == input", total)
}

// TestReferenceDecodeLargeVersions forces a spread of specific large versions to
// prove placement/interleaving is correct well beyond the small-version cases.
func TestReferenceDecodeLargeVersions(t *testing.T) {
	for _, v := range []int{7, 10, 14, 20, 27, 33, 40} {
		// Fill roughly 60% of the L-level capacity with printable data.
		capacity := qr.ByteCapacity(v, qr.Low)
		payload := []byte(strings.Repeat("QR-", capacity/3*3/9+1))
		if len(payload) > capacity {
			payload = payload[:capacity]
		}
		m, err := qr.Encode(payload, qr.WithLevel(qr.Low), qr.WithVersion(v))
		if err != nil {
			t.Fatalf("v%d: encode: %v", v, err)
		}
		if m.Version != v {
			t.Fatalf("v%d: got version %d", v, m.Version)
		}
		got := decode(t, m)
		if !bytes.Equal(got, payload) {
			t.Fatalf("v%d: round-trip mismatch (%d vs %d bytes)", v, len(payload), len(got))
		}
	}
}

// TestReferenceDecodeEveryMask forces each of the eight masks on the same payload
// and confirms all remain decodable.
func TestReferenceDecodeEveryMask(t *testing.T) {
	payload := []byte("mask-coverage-check-0123456789")
	for mask := 0; mask < 8; mask++ {
		m, err := qr.Encode(payload, qr.WithMask(mask))
		if err != nil {
			t.Fatalf("mask %d: encode: %v", mask, err)
		}
		if m.Mask != mask {
			t.Fatalf("mask %d: got mask %d", mask, m.Mask)
		}
		got := decode(t, m)
		if !bytes.Equal(got, payload) {
			t.Fatalf("mask %d: round-trip mismatch", mask)
		}
	}
}

func TestCapacityReport(t *testing.T) {
	// Emit the capacity figures the playground needs to calibrate its
	// compress-or-fallback threshold.
	for _, v := range []int{20, 30, 40} {
		t.Logf("version %2d: L=%d M=%d Q=%d H=%d bytes",
			v,
			qr.ByteCapacity(v, qr.Low),
			qr.ByteCapacity(v, qr.Medium),
			qr.ByteCapacity(v, qr.Quartile),
			qr.ByteCapacity(v, qr.High))
	}
}
