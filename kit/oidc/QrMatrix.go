package oidc

import "slices"

type QrMatrix struct {
	size     int
	modules  []int
	reserved []int
}

func (QrMatrix) sized(size int) QrMatrix {
	return QrMatrix{size: size, modules: make([]int, size*size), reserved: make([]int, size*size)}
}

func (matrix QrMatrix) clone() QrMatrix {
	return QrMatrix{size: matrix.size, modules: slices.Clone(matrix.modules), reserved: slices.Clone(matrix.reserved)}
}

func (matrix QrMatrix) index(position QrPosition) int {
	return position.row*matrix.size + position.column
}

func (matrix QrMatrix) module(position QrPosition) int {
	return matrix.modules[matrix.index(position)]
}

func (matrix QrMatrix) isReserved(position QrPosition) bool {
	return matrix.reserved[matrix.index(position)] == 1
}

func (matrix QrMatrix) freeFlag(index int) int {
	return 1 - matrix.reserved[index]
}

func (matrix QrMatrix) setData(position QrPosition, value int) {
	matrix.modules[matrix.index(position)] = value
}

func (matrix QrMatrix) setFunction(position QrPosition, value int) {
	matrix.modules[matrix.index(position)] = value
	matrix.reserved[matrix.index(position)] = 1
}

func (matrix QrMatrix) clamp(position QrPosition) QrPosition {
	last := matrix.size - 1
	return QrPosition{row: min(max(position.row, 0), last), column: min(max(position.column, 0), last)}
}
