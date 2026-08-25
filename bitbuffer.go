package qr

// bitBuffer accumulates bits most-significant-first and packs them into bytes.
type bitBuffer struct {
	bits []bool
}

// appendBits appends the low n bits of val, most-significant bit first.
// It requires 0 <= n <= 31 and val to fit in n bits.
func (b *bitBuffer) appendBits(val, n int) {
	for i := n - 1; i >= 0; i-- {
		b.bits = append(b.bits, (val>>uint(i))&1 == 1)
	}
}

// len returns the number of buffered bits.
func (b *bitBuffer) len() int { return len(b.bits) }

// bytes packs the buffered bits into bytes. The bit count is always a multiple
// of eight by the time this is called.
func (b *bitBuffer) bytes() []byte {
	out := make([]byte, len(b.bits)/8)
	for i, bit := range b.bits {
		if bit {
			out[i>>3] |= 1 << uint(7-i&7)
		}
	}
	return out
}
