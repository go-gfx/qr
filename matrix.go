package qr

// This file draws the QR Code symbol: the function patterns (finders, timing,
// alignment, format/version reservations), the interleaved codeword bitstream in
// its zig-zag walk, the eight data masks, their penalty scoring and selection,
// and the format/version information. It follows ISO/IEC 18004 module placement.

// drawFunctionPatterns draws every function module and reserves (with a
// placeholder) the format and version information areas. isFunction is marked
// true for every module the codeword walk must skip.
func (m *Matrix) drawFunctionPatterns(version int, isFunction []bool) {
	size := m.dim
	setFn := func(x, y int, dark bool) {
		m.set(x, y, dark)
		isFunction[y*size+x] = true
	}

	// Timing patterns.
	for i := 0; i < size; i++ {
		setFn(6, i, i%2 == 0)
		setFn(i, 6, i%2 == 0)
	}

	// Finder patterns and their separators, at the three corners.
	m.drawFinderPattern(3, 3, setFn)
	m.drawFinderPattern(size-4, 3, setFn)
	m.drawFinderPattern(3, size-4, setFn)

	// Alignment patterns, skipping the three that overlap the finders.
	pos := alignmentPatternPositions(version)
	n := len(pos)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if (i == 0 && j == 0) || (i == 0 && j == n-1) || (i == n-1 && j == 0) {
				continue
			}
			m.drawAlignmentPattern(pos[i], pos[j], setFn)
		}
	}

	// Reserve the format and version information areas.
	m.drawFormatBits(0, isFunction)
	m.drawVersion(version, setFn)
}

// drawFinderPattern draws a 7x7 finder pattern (with its 1-module separator)
// centred at (cx, cy).
func (m *Matrix) drawFinderPattern(cx, cy int, setFn func(x, y int, dark bool)) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			dist := abs(dx)
			if a := abs(dy); a > dist {
				dist = a
			}
			x, y := cx+dx, cy+dy
			if x >= 0 && x < m.dim && y >= 0 && y < m.dim {
				setFn(x, y, dist != 2 && dist != 4)
			}
		}
	}
}

// drawAlignmentPattern draws a 5x5 alignment pattern centred at (cx, cy).
func (m *Matrix) drawAlignmentPattern(cx, cy int, setFn func(x, y int, dark bool)) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			dist := abs(dx)
			if a := abs(dy); a > dist {
				dist = a
			}
			setFn(cx+dx, cy+dy, dist != 1)
		}
	}
}

// drawVersion draws the 18-bit version information (versions 7..40 only) into its
// two reserved blocks.
func (m *Matrix) drawVersion(version int, setFn func(x, y int, dark bool)) {
	if version < 7 {
		return
	}
	// BCH(18,6) with generator 0x1F25.
	rem := version
	for i := 0; i < 12; i++ {
		rem = (rem << 1) ^ ((rem >> 11) * 0x1F25)
	}
	bits := version<<12 | rem
	size := m.dim
	for i := 0; i < 18; i++ {
		bit := (bits>>uint(i))&1 == 1
		a, b := size-11+i%3, i/3
		setFn(a, b, bit)
		setFn(b, a, bit)
	}
}

// drawFormatBits draws the 15-bit format information for the given mask into its
// two reserved blocks, and sets the always-dark module. It is called first with
// a placeholder mask to reserve (mark as function) the area, then again with the
// chosen mask to write the final bits.
func (m *Matrix) drawFormatBits(mask int, isFunction []bool) {
	size := m.dim
	setFn := func(x, y int, dark bool) {
		m.set(x, y, dark)
		isFunction[y*size+x] = true
	}
	// Compute the 15-bit value: 2 EC-level bits + 3 mask bits + 10 BCH bits.
	data := m.Level.formatBits()<<3 | mask
	rem := data
	for i := 0; i < 10; i++ {
		rem = (rem << 1) ^ ((rem >> 9) * 0x537)
	}
	bits := (data<<10 | rem) ^ 0x5412

	// First copy near the top-left finder.
	for i := 0; i <= 5; i++ {
		setFn(8, i, getBit(bits, i))
	}
	setFn(8, 7, getBit(bits, 6))
	setFn(8, 8, getBit(bits, 7))
	setFn(7, 8, getBit(bits, 8))
	for i := 9; i < 15; i++ {
		setFn(14-i, 8, getBit(bits, i))
	}

	// Second copy split across the other two finders.
	for i := 0; i < 8; i++ {
		setFn(size-1-i, 8, getBit(bits, i))
	}
	for i := 8; i < 15; i++ {
		setFn(8, size-15+i, getBit(bits, i))
	}
	setFn(8, size-8, true) // always-dark module
}

// drawCodewords places the interleaved codeword sequence into the matrix,
// following the upward/downward zig-zag over the two-module-wide columns and
// skipping function modules.
func (m *Matrix) drawCodewords(data []byte, isFunction []bool) {
	size := m.dim
	i := 0 // bit index into data
	for right := size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5 // skip the vertical timing column
		}
		for vert := 0; vert < size; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				upward := (right+1)&2 == 0
				y := vert
				if upward {
					y = size - 1 - vert
				}
				if !isFunction[y*size+x] && i < len(data)*8 {
					m.set(x, y, getBit(int(data[i>>3]), 7-i&7))
					i++
				}
			}
		}
	}
}

// applyBestMask applies a data mask and returns the mask index used. If forced is
// in 0..7 that mask is applied; otherwise all eight are scored by the standard
// penalty and the lowest-penalty mask is chosen.
func (m *Matrix) applyBestMask(forced int, isFunction []bool) int {
	if forced >= 0 && forced <= 7 {
		m.applyMask(forced, isFunction)
		m.drawFormatBits(forced, isFunction)
		return forced
	}
	best := 0
	minPenalty := int(^uint(0) >> 1) // max int
	for mask := 0; mask < 8; mask++ {
		m.applyMask(mask, isFunction)
		m.drawFormatBits(mask, isFunction)
		p := m.penaltyScore()
		if p < minPenalty {
			minPenalty = p
			best = mask
		}
		m.applyMask(mask, isFunction) // undo (XOR is its own inverse)
	}
	m.applyMask(best, isFunction)
	m.drawFormatBits(best, isFunction)
	return best
}

// applyMask XORs the given mask over every non-function module.
func (m *Matrix) applyMask(mask int, isFunction []bool) {
	size := m.dim
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if isFunction[y*size+x] {
				continue
			}
			if maskBit(mask, x, y) {
				m.set(x, y, !m.at(x, y))
			}
		}
	}
}

// maskBit reports whether the module at (x, y) is inverted by the given mask.
func maskBit(mask, x, y int) bool {
	switch mask {
	case 0:
		return (x+y)%2 == 0
	case 1:
		return y%2 == 0
	case 2:
		return x%3 == 0
	case 3:
		return (x+y)%3 == 0
	case 4:
		return (y/2+x/3)%2 == 0
	case 5:
		return x*y%2+x*y%3 == 0
	case 6:
		return (x*y%2+x*y%3)%2 == 0
	default: // 7
		return ((x+y)%2+x*y%3)%2 == 0
	}
}

func getBit(x, i int) bool { return (x>>uint(i))&1 == 1 }

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
