package qr

// This file holds the fixed characteristics of every QR version/error-correction
// combination, taken from ISO/IEC 18004. Rather than transcribe the full
// 160-entry block table, only two arrays are stored per error-correction level:
// the number of error-correction codewords per block and the number of blocks.
// Everything else (raw codeword count, data-codeword count, short/long block
// split) is derived, which keeps the risk of a transcription error to two
// well-known vectors that the reference-decoder round-trip test exercises end
// to end.

// eccCodewordsPerBlock[level][version] is the number of Reed–Solomon error
// correction codewords in every block. Index 0 (version 0) is unused.
var eccCodewordsPerBlock = [4][41]int{
	// L
	{-1, 7, 10, 15, 20, 26, 18, 20, 24, 30, 18, 20, 24, 26, 30, 22, 24, 28, 30, 28, 28, 28, 28, 30, 30, 26, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
	// M
	{-1, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26, 26, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28},
	// Q
	{-1, 13, 22, 18, 26, 18, 24, 18, 22, 20, 24, 28, 26, 24, 20, 30, 24, 28, 28, 26, 30, 28, 30, 30, 30, 30, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
	// H
	{-1, 17, 28, 22, 16, 22, 28, 26, 26, 24, 28, 24, 28, 22, 24, 24, 30, 28, 28, 26, 28, 30, 24, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
}

// numErrorCorrectionBlocks[level][version] is the number of blocks the data is
// split into. Index 0 (version 0) is unused.
var numErrorCorrectionBlocks = [4][41]int{
	// L
	{-1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4, 4, 4, 4, 4, 6, 6, 6, 6, 7, 8, 8, 9, 9, 10, 12, 12, 12, 13, 14, 15, 16, 17, 18, 19, 19, 20, 21, 22, 24, 25},
	// M
	{-1, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17, 17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33, 35, 37, 38, 40, 43, 45, 47, 49},
	// Q
	{-1, 1, 1, 2, 2, 4, 4, 6, 6, 8, 8, 8, 10, 12, 16, 12, 17, 16, 18, 21, 20, 23, 23, 25, 27, 29, 34, 34, 35, 38, 40, 43, 45, 48, 51, 53, 56, 59, 62, 65, 68},
	// H
	{-1, 1, 1, 2, 4, 4, 4, 5, 6, 8, 8, 11, 11, 16, 16, 18, 16, 19, 21, 25, 25, 25, 34, 30, 32, 35, 37, 40, 42, 45, 48, 51, 54, 57, 60, 63, 66, 70, 74, 77, 81},
}

// numRawDataModules returns the number of data modules (i.e. modules available
// for data and error-correction codewords, before the remainder bits) for the
// given version. Function patterns and format/version information are excluded.
func numRawDataModules(version int) int {
	result := (16*version+128)*version + 64
	if version >= 2 {
		numAlign := version/7 + 2
		result -= (25*numAlign-10)*numAlign - 55
		if version >= 7 {
			result -= 36
		}
	}
	return result
}

// numRawCodewords returns the number of 8-bit codewords (data + EC, excluding
// the trailing remainder bits) that fit in the given version.
func numRawCodewords(version int) int {
	return numRawDataModules(version) / 8
}

// numDataCodewords returns the number of data codewords (excluding EC) available
// at the given version and error-correction level.
func numDataCodewords(version int, level ECLevel) int {
	ecc := eccCodewordsPerBlock[level][version]
	blocks := numErrorCorrectionBlocks[level][version]
	return numRawCodewords(version) - ecc*blocks
}

// alignmentPatternPositions returns the row/column coordinates of the alignment
// pattern centres for the given version (empty for version 1).
func alignmentPatternPositions(version int) []int {
	if version == 1 {
		return nil
	}
	numAlign := version/7 + 2
	var step int
	if version == 32 {
		step = 26
	} else {
		step = (version*4 + numAlign*2 + 1) / (numAlign*2 - 2) * 2
	}
	result := make([]int, numAlign)
	result[0] = 6
	for i, pos := numAlign-1, version*4+10; i >= 1; i, pos = i-1, pos-step {
		result[i] = pos
	}
	return result
}
