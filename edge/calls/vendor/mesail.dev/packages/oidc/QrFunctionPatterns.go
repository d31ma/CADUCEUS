package oidc

type QrFunctionPatterns struct{}

func (QrFunctionPatterns) finderCenter() int {
	return 3
}

func (QrFunctionPatterns) finderRings() []int {
	return []int{1, 1, 0, 1, 0}
}

func (QrFunctionPatterns) alignmentRings() []int {
	return []int{1, 0, 1}
}

func (QrFunctionPatterns) timingLine() int {
	return 6
}

func (QrFunctionPatterns) timingPeriod() int {
	return 2
}

func (QrFunctionPatterns) edgeOffset() int {
	return 8
}

func (patterns QrFunctionPatterns) draw(matrix QrMatrix, version int) {
	patterns.drawFinders(matrix)
	patterns.drawAlignments(matrix, QrVersionTable{}.alignmentCenters(version))
	patterns.drawTiming(matrix)
	patterns.drawDarkModule(matrix)
	information := QrFormatInformation{}
	information.drawFormat(matrix, 0)
	information.drawVersion(matrix, version)
}

func (patterns QrFunctionPatterns) drawFinders(matrix QrMatrix) {
	near := patterns.finderCenter()
	far := matrix.size - 1 - patterns.finderCenter()
	rings := patterns.finderRings()
	patterns.drawSquare(matrix, QrPosition{row: near, column: near}, rings)
	patterns.drawSquare(matrix, QrPosition{row: near, column: far}, rings)
	patterns.drawSquare(matrix, QrPosition{row: far, column: near}, rings)
}

func (patterns QrFunctionPatterns) drawSquare(matrix QrMatrix, center QrPosition, rings []int) {
	radius := len(rings) - 1
	first := matrix.clamp(QrPosition{row: center.row - radius, column: center.column - radius})
	last := matrix.clamp(QrPosition{row: center.row + radius, column: center.column + radius})
	width := last.column - first.column + 1
	for index := range width * (last.row - first.row + 1) {
		cell := QrPosition{row: first.row + index/width, column: first.column + index%width}
		matrix.setFunction(cell, rings[patterns.distance(center, cell)])
	}
}

func (QrFunctionPatterns) distance(center QrPosition, cell QrPosition) int {
	return max(center.row-cell.row, cell.row-center.row, center.column-cell.column, cell.column-center.column)
}

func (patterns QrFunctionPatterns) drawAlignments(matrix QrMatrix, centers []int) {
	count := len(centers)
	for index := range count * count {
		patterns.drawAlignmentIfFree(matrix, QrPosition{row: centers[index/count], column: centers[index%count]})
	}
}

func (patterns QrFunctionPatterns) drawAlignmentIfFree(matrix QrMatrix, center QrPosition) {
	if matrix.isReserved(center) {
		return
	}
	patterns.drawSquare(matrix, center, patterns.alignmentRings())
}

func (patterns QrFunctionPatterns) drawTiming(matrix QrMatrix) {
	line := patterns.timingLine()
	for index := patterns.edgeOffset(); index < matrix.size-patterns.edgeOffset(); index++ {
		value := 1 - index%patterns.timingPeriod()
		matrix.setFunction(QrPosition{row: line, column: index}, value)
		matrix.setFunction(QrPosition{row: index, column: line}, value)
	}
}

func (patterns QrFunctionPatterns) drawDarkModule(matrix QrMatrix) {
	matrix.setFunction(QrPosition{row: matrix.size - patterns.edgeOffset(), column: patterns.edgeOffset()}, 1)
}
