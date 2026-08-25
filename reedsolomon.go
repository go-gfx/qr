package qr

// Reed–Solomon error correction over GF(256) with the QR Code field-generator
// polynomial 0x11D (x^8 + x^4 + x^3 + x^2 + 1), matching ISO/IEC 18004.

// gfMultiply returns the product of two GF(256) elements.
func gfMultiply(x, y byte) byte {
	z := 0
	for i := 7; i >= 0; i-- {
		z = (z << 1) ^ ((z >> 7) * 0x11D)
		z ^= int((y>>uint(i))&1) * int(x)
	}
	return byte(z)
}

// rsComputeDivisor returns the coefficients of the Reed–Solomon generator
// (divisor) polynomial of the given degree, highest power first (excluding the
// leading 1 term). degree must be between 1 and 255.
func rsComputeDivisor(degree int) []byte {
	result := make([]byte, degree)
	result[degree-1] = 1 // start off with the monomial x^0
	// Compute the product polynomial (x - r^0)(x - r^1)...(x - r^(degree-1)),
	// and drop the highest monomial term which is always 1.
	root := byte(1)
	for i := 0; i < degree; i++ {
		for j := 0; j < len(result); j++ {
			result[j] = gfMultiply(result[j], root)
			if j+1 < len(result) {
				result[j] ^= result[j+1]
			}
		}
		root = gfMultiply(root, 0x02)
	}
	return result
}

// rsComputeRemainder returns the Reed–Solomon error-correction codewords for the
// given data using the supplied divisor polynomial.
func rsComputeRemainder(data, divisor []byte) []byte {
	result := make([]byte, len(divisor))
	for _, b := range data {
		factor := b ^ result[0]
		copy(result, result[1:])
		result[len(result)-1] = 0
		for i := 0; i < len(result); i++ {
			result[i] ^= gfMultiply(divisor[i], factor)
		}
	}
	return result
}
