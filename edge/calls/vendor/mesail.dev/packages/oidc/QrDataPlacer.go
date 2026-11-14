package oidc

type QrDataPlacer struct{}

func (QrDataPlacer) pairWidth() int {
	return 2
}

func (QrDataPlacer) timingColumn() int {
	return 6
}

func (placer QrDataPlacer) place(matrix QrMatrix, codewords []byte) {
	buffer := &QrBitBuffer{}
	buffer.appendBytes(codewords)
	positions := placer.freePositions(matrix)
	for index := range min(len(positions), buffer.length()) {
		matrix.setData(positions[index], buffer.bits[index])
	}
}

func (placer QrDataPlacer) freePositions(matrix QrMatrix) []QrPosition {
	result := []QrPosition{}
	pairLength := placer.pairWidth() * matrix.size
	for index := range placer.pairCount(matrix.size) * pairLength {
		result = placer.appendIfFree(result, matrix, placer.positionAt(index, matrix.size))
	}
	return result
}

func (placer QrDataPlacer) pairCount(size int) int {
	return (size - 1) / placer.pairWidth()
}

func (placer QrDataPlacer) positionAt(index int, size int) QrPosition {
	pairLength := placer.pairWidth() * size
	pair := index / pairLength
	step := index % pairLength
	return QrPosition{
		row:    placer.row(pair, step/placer.pairWidth(), size),
		column: placer.rightColumn(pair, size) - step%placer.pairWidth(),
	}
}

func (placer QrDataPlacer) row(pair int, vertical int, size int) int {
	if pair%placer.pairWidth() == 0 {
		return size - 1 - vertical
	}
	return vertical
}

func (placer QrDataPlacer) rightColumn(pair int, size int) int {
	column := size - 1 - placer.pairWidth()*pair
	if column > placer.timingColumn() {
		return column
	}
	return column - 1
}

func (QrDataPlacer) appendIfFree(result []QrPosition, matrix QrMatrix, position QrPosition) []QrPosition {
	if matrix.isReserved(position) {
		return result
	}
	return append(result, position)
}
