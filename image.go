package qr

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// Image renders the matrix to a paletted image.Image at the given module scale
// (pixels per module) and with an extra quiet zone of quietZone modules added
// around the code's own quiet zone. Dark modules are black, light modules white.
//
// scale is clamped to at least 1 and quietZone to at least 0. The helper depends
// only on the standard library, so importing qr for the matrix alone pulls in no
// image code beyond what the standard library already provides.
func (m *Matrix) Image(scale, quietZone int) image.Image {
	if scale < 1 {
		scale = 1
	}
	if quietZone < 0 {
		quietZone = 0
	}
	modules := m.Size() + 2*quietZone
	side := modules * scale
	img := image.NewPaletted(image.Rect(0, 0, side, side), color.Palette{color.White, color.Black})
	// Background is palette index 0 (white) already; fill only the dark modules.
	for my := 0; my < m.Size(); my++ {
		for mx := 0; mx < m.Size(); mx++ {
			if !m.Module(mx, my) {
				continue
			}
			x0 := (mx + quietZone) * scale
			y0 := (my + quietZone) * scale
			for dy := 0; dy < scale; dy++ {
				row := img.Pix[(y0+dy)*img.Stride+x0 : (y0+dy)*img.Stride+x0+scale]
				for i := range row {
					row[i] = 1 // black
				}
			}
		}
	}
	return img
}

// pngEncode is the PNG encoder seam, overridable in tests to exercise the error
// path (png.Encode cannot fail for the valid paletted images Image produces).
var pngEncode = png.Encode

// PNG renders the matrix with Image and encodes it as a PNG. See Image for the
// meaning of scale and quietZone.
func (m *Matrix) PNG(scale, quietZone int) ([]byte, error) {
	var buf bytes.Buffer
	if err := pngEncode(&buf, m.Image(scale, quietZone)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
