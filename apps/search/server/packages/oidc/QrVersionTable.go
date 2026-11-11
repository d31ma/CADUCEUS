package oidc

type QrVersionTable struct{}

func (QrVersionTable) minimumVersion() int {
	return 1
}

func (QrVersionTable) maximumVersion() int {
	return 10
}

func (QrVersionTable) baseSize() int {
	return 17
}

func (QrVersionTable) sizeStep() int {
	return 4
}

func (table QrVersionTable) size(version int) int {
	return table.baseSize() + table.sizeStep()*version
}

func (table QrVersionTable) index(version int) int {
	return version - table.minimumVersion()
}

func (table QrVersionTable) dataCodewords(version int) int {
	return table.dataCodewordCounts()[table.index(version)]
}

func (table QrVersionTable) correctionCodewordsPerBlock(version int) int {
	return table.correctionCodewordCounts()[table.index(version)]
}

func (table QrVersionTable) blockCount(version int) int {
	return table.blockCounts()[table.index(version)]
}

func (table QrVersionTable) alignmentCenters(version int) []int {
	return table.alignmentCenterLists()[table.index(version)]
}

func (table QrVersionTable) countBits(version int) int {
	return table.countBitLengths()[table.index(version)]
}

func (QrVersionTable) dataCodewordCounts() []int {
	return []int{16, 28, 44, 64, 86, 108, 124, 154, 182, 216}
}

func (QrVersionTable) correctionCodewordCounts() []int {
	return []int{10, 16, 26, 18, 24, 16, 18, 22, 22, 26}
}

func (QrVersionTable) blockCounts() []int {
	return []int{1, 1, 1, 2, 2, 4, 4, 4, 5, 5}
}

func (QrVersionTable) countBitLengths() []int {
	return []int{8, 8, 8, 8, 8, 8, 8, 8, 8, 16}
}

func (QrVersionTable) alignmentCenterLists() [][]int {
	return [][]int{
		{},
		{6, 18},
		{6, 22},
		{6, 26},
		{6, 30},
		{6, 34},
		{6, 22, 38},
		{6, 24, 42},
		{6, 26, 46},
		{6, 28, 50},
	}
}
