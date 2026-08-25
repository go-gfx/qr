package qr

// Data-mask penalty scoring per ISO/IEC 18004 §8.8.2. Lower is better; the mask
// with the lowest total is chosen when selection is automatic.
const (
	penaltyN1 = 3
	penaltyN2 = 3
	penaltyN3 = 40
	penaltyN4 = 10
)

// penaltyScore returns the total penalty of the current (masked) matrix.
func (m *Matrix) penaltyScore() int {
	size := m.dim
	result := 0

	// Rule 1 (rows) and rule 3 (finder-like patterns) in the horizontal direction.
	for y := 0; y < size; y++ {
		runColor := false
		runLen := 0
		var history [7]int
		for x := 0; x < size; x++ {
			if m.at(x, y) == runColor {
				runLen++
				if runLen == 5 {
					result += penaltyN1
				} else if runLen > 5 {
					result++
				}
			} else {
				m.finderPenaltyAddHistory(runLen, &history)
				if !runColor {
					result += m.finderPenaltyCountPatterns(&history) * penaltyN3
				}
				runColor = m.at(x, y)
				runLen = 1
			}
		}
		result += m.finderPenaltyTerminateAndCount(runColor, runLen, &history) * penaltyN3
	}
	// Rule 1 (columns) and rule 3 in the vertical direction.
	for x := 0; x < size; x++ {
		runColor := false
		runLen := 0
		var history [7]int
		for y := 0; y < size; y++ {
			if m.at(x, y) == runColor {
				runLen++
				if runLen == 5 {
					result += penaltyN1
				} else if runLen > 5 {
					result++
				}
			} else {
				m.finderPenaltyAddHistory(runLen, &history)
				if !runColor {
					result += m.finderPenaltyCountPatterns(&history) * penaltyN3
				}
				runColor = m.at(x, y)
				runLen = 1
			}
		}
		result += m.finderPenaltyTerminateAndCount(runColor, runLen, &history) * penaltyN3
	}

	// Rule 2: 2x2 blocks of the same colour.
	for y := 0; y < size-1; y++ {
		for x := 0; x < size-1; x++ {
			c := m.at(x, y)
			if c == m.at(x+1, y) && c == m.at(x, y+1) && c == m.at(x+1, y+1) {
				result += penaltyN2
			}
		}
	}

	// Rule 4: deviation of the dark-module proportion from 50%.
	dark := 0
	for _, v := range m.modules {
		if v {
			dark++
		}
	}
	total := size * size
	k := (abs(dark*20-total*10)+total-1)/total - 1
	result += k * penaltyN4
	return result
}

// finderPenaltyCountPatterns counts the finder-like 1:1:3:1:1 patterns in the
// run-length history, each surrounded by a run of light modules at least four
// times the unit width on one side.
func (m *Matrix) finderPenaltyCountPatterns(history *[7]int) int {
	n := history[1]
	core := n > 0 && history[2] == n && history[3] == n*3 && history[4] == n && history[5] == n
	count := 0
	if core && history[0] >= n*4 && history[6] >= n {
		count++
	}
	if core && history[6] >= n*4 && history[0] >= n {
		count++
	}
	return count
}

// finderPenaltyAddHistory pushes a new run length onto the history, padding the
// very first run with the implicit light border so the finder test can match at
// the symbol edge.
func (m *Matrix) finderPenaltyAddHistory(runLen int, history *[7]int) {
	if history[0] == 0 {
		runLen += m.dim // add light border to the first run
	}
	copy(history[1:], history[:6])
	history[0] = runLen
}

// finderPenaltyTerminateAndCount flushes the final run (adding the trailing light
// border) and returns the finder-pattern count.
func (m *Matrix) finderPenaltyTerminateAndCount(runColor bool, runLen int, history *[7]int) int {
	if runColor { // ended on a dark run
		m.finderPenaltyAddHistory(runLen, history)
		runLen = 0
	}
	runLen += m.dim // add light border to the last run
	m.finderPenaltyAddHistory(runLen, history)
	return m.finderPenaltyCountPatterns(history)
}
