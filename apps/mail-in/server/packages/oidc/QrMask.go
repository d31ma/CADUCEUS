package oidc

type QrMask struct{}

func (QrMask) count() int {
	return 8
}

func (QrMask) pairPeriod() int {
	return 2
}

func (QrMask) triplePeriod() int {
	return 3
}

func (mask QrMask) apply(matrix QrMatrix, number int) {
	pattern := mask.patterns()[number]
	for index := range matrix.modules {
		flip := mask.flipWhenZero(pattern(index/matrix.size, index%matrix.size))
		matrix.modules[index] ^= flip * matrix.freeFlag(index)
	}
}

func (QrMask) flipWhenZero(value int) int {
	return 1 - min(1, value)
}

func (mask QrMask) patterns() []func(row int, column int) int {
	return []func(row int, column int) int{
		mask.checkerboard,
		mask.rowStripes,
		mask.columnStripes,
		mask.diagonalStripes,
		mask.blocks,
		mask.productCells,
		mask.productDiamonds,
		mask.mixedDiamonds,
	}
}

func (mask QrMask) checkerboard(row int, column int) int {
	return (row + column) % mask.pairPeriod()
}

func (mask QrMask) rowStripes(row int, _ int) int {
	return row % mask.pairPeriod()
}

func (mask QrMask) columnStripes(_ int, column int) int {
	return column % mask.triplePeriod()
}

func (mask QrMask) diagonalStripes(row int, column int) int {
	return (row + column) % mask.triplePeriod()
}

func (mask QrMask) blocks(row int, column int) int {
	return (row/mask.pairPeriod() + column/mask.triplePeriod()) % mask.pairPeriod()
}

func (mask QrMask) productCells(row int, column int) int {
	return (row*column)%mask.pairPeriod() + (row*column)%mask.triplePeriod()
}

func (mask QrMask) productDiamonds(row int, column int) int {
	return mask.productCells(row, column) % mask.pairPeriod()
}

func (mask QrMask) mixedDiamonds(row int, column int) int {
	return ((row+column)%mask.pairPeriod() + (row*column)%mask.triplePeriod()) % mask.pairPeriod()
}
