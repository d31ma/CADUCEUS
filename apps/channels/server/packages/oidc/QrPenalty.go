package oidc

import "slices"

type QrPenalty struct{}

func (penalty QrPenalty) score(matrix QrMatrix) int {
	lines := append(penalty.rows(matrix), penalty.columns(matrix)...)
	return penalty.runScore(lines) + penalty.blockScore(matrix) + penalty.finderScore(lines) + penalty.balanceScore(matrix)
}

func (QrPenalty) rows(matrix QrMatrix) [][]int {
	result := [][]int{}
	for row := range matrix.size {
		result = append(result, matrix.modules[row*matrix.size:(row+1)*matrix.size])
	}
	return result
}

func (QrPenalty) columns(matrix QrMatrix) [][]int {
	result := make([][]int, matrix.size)
	for index, value := range matrix.modules {
		column := index % matrix.size
		result[column] = append(result[column], value)
	}
	return result
}

func (penalty QrPenalty) runScore(lines [][]int) int {
	total := 0
	for _, line := range lines {
		total += penalty.lineRunScore(line)
	}
	return total
}

func (penalty QrPenalty) lineRunScore(line []int) int {
	total := 0
	for _, run := range penalty.runs(line) {
		total += penalty.runPenalty(run)
	}
	return total
}

func (penalty QrPenalty) runs(line []int) []int {
	result := []int{1}
	for index := 1; index < len(line); index++ {
		result = penalty.extendRuns(result, line[index-1] == line[index])
	}
	return result
}

func (QrPenalty) extendRuns(runs []int, same bool) []int {
	if same {
		runs[len(runs)-1]++
		return runs
	}
	return append(runs, 1)
}

func (penalty QrPenalty) runPenalty(run int) int {
	if run < penalty.values().RunMinimum {
		return 0
	}
	return penalty.values().RunBase + run - penalty.values().RunMinimum
}

func (penalty QrPenalty) blockScore(matrix QrMatrix) int {
	width := matrix.size - 1
	total := 0
	for index := range width * width {
		total += penalty.blockPenalty(matrix, QrPosition{row: index / width, column: index % width})
	}
	return total
}

func (penalty QrPenalty) blockPenalty(matrix QrMatrix, corner QrPosition) int {
	if penalty.blockSum(matrix, corner)%penalty.values().BlockArea != 0 {
		return 0
	}
	return penalty.values().BlockWeight
}

func (QrPenalty) blockSum(matrix QrMatrix, corner QrPosition) int {
	below := corner.row + 1
	right := corner.column + 1
	return matrix.module(corner) +
		matrix.module(QrPosition{row: corner.row, column: right}) +
		matrix.module(QrPosition{row: below, column: corner.column}) +
		matrix.module(QrPosition{row: below, column: right})
}

func (penalty QrPenalty) finderScore(lines [][]int) int {
	total := 0
	for _, line := range lines {
		total += penalty.lineFinderScore(penalty.withQuietZone(line))
	}
	return total
}

func (penalty QrPenalty) withQuietZone(line []int) []int {
	quiet := make([]int, penalty.values().QuietWidth)
	return slices.Concat(quiet, line, quiet)
}

func (penalty QrPenalty) lineFinderScore(line []int) int {
	width := len(penalty.values().FinderBefore)
	total := 0
	for offset := range len(line) - width + 1 {
		window := line[offset : offset+width]
		total += penalty.matchPenalty(window, penalty.values().FinderBefore) + penalty.matchPenalty(window, penalty.values().FinderAfter)
	}
	return total
}

func (penalty QrPenalty) matchPenalty(window []int, pattern []int) int {
	if !slices.Equal(window, pattern) {
		return 0
	}
	return penalty.values().FinderWeight
}

func (penalty QrPenalty) balanceScore(matrix QrMatrix) int {
	total := len(matrix.modules)
	dark := penalty.darkCount(matrix)
	light := total - dark
	difference := max(dark-light, light-dark)
	return penalty.values().BalanceWeight * (difference * penalty.values().BalanceSteps / total)
}

func (QrPenalty) darkCount(matrix QrMatrix) int {
	total := 0
	for _, value := range matrix.modules {
		total += value
	}
	return total
}

func (QrPenalty) values() QrPenaltyValues {
	return QrPenaltyValues{
		RunMinimum:    5,
		RunBase:       3,
		BlockArea:     4,
		BlockWeight:   3,
		FinderWeight:  40,
		QuietWidth:    4,
		BalanceSteps:  10,
		BalanceWeight: 10,
		FinderBefore:  []int{0, 0, 0, 0, 1, 0, 1, 1, 1, 0, 1},
		FinderAfter:   []int{1, 0, 1, 1, 1, 0, 1, 0, 0, 0, 0},
	}
}
