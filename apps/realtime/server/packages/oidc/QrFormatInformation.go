package oidc

type QrFormatInformation struct{}

func (information QrFormatInformation) formatBits(mask int) int {
	data := information.values().LevelBits<<information.values().MaskBits | mask
	code := QrBchCode{}.encode(data, information.values().FormatDegree, information.values().FormatGenerator)
	return code ^ information.values().FormatMask
}

func (information QrFormatInformation) versionBits(version int) int {
	return QrBchCode{}.encode(version, information.values().VersionDegree, information.values().VersionGenerator)
}

func (information QrFormatInformation) drawFormat(matrix QrMatrix, mask int) {
	bits := information.formatBits(mask)
	information.drawBits(matrix, bits, information.firstFormatPositions())
	information.drawBits(matrix, bits, information.secondFormatPositions(matrix.size))
}

func (QrFormatInformation) drawBits(matrix QrMatrix, bits int, positions []QrPosition) {
	for index, position := range positions {
		matrix.setFunction(position, (bits>>index)&1)
	}
}

func (QrFormatInformation) firstFormatPositions() []QrPosition {
	return []QrPosition{
		{row: 0, column: 8}, {row: 1, column: 8}, {row: 2, column: 8}, {row: 3, column: 8},
		{row: 4, column: 8}, {row: 5, column: 8}, {row: 7, column: 8}, {row: 8, column: 8},
		{row: 8, column: 7}, {row: 8, column: 5}, {row: 8, column: 4}, {row: 8, column: 3},
		{row: 8, column: 2}, {row: 8, column: 1}, {row: 8, column: 0},
	}
}

func (information QrFormatInformation) secondFormatPositions(size int) []QrPosition {
	return append(information.topRightPositions(size), information.bottomLeftPositions(size)...)
}

func (information QrFormatInformation) topRightPositions(size int) []QrPosition {
	result := []QrPosition{}
	for index := range information.values().FormatLine {
		result = append(result, QrPosition{row: information.values().FormatLine, column: size - 1 - index})
	}
	return result
}

func (information QrFormatInformation) bottomLeftPositions(size int) []QrPosition {
	result := []QrPosition{}
	for index := information.values().FormatLine; index < information.values().FormatLength; index++ {
		result = append(result, QrPosition{row: size - information.values().FormatLength + index, column: information.values().FormatLine})
	}
	return result
}

func (information QrFormatInformation) drawVersion(matrix QrMatrix, version int) {
	if version < information.values().VersionMinimum {
		return
	}
	information.drawVersionBits(matrix, information.versionBits(version))
}

func (information QrFormatInformation) drawVersionBits(matrix QrMatrix, bits int) {
	width := information.values().VersionAreaWidth
	for index := range information.values().VersionLength {
		near := index / width
		far := matrix.size - information.values().VersionAreaOffset + index%width
		value := (bits >> index) & 1
		matrix.setFunction(QrPosition{row: near, column: far}, value)
		matrix.setFunction(QrPosition{row: far, column: near}, value)
	}
}

func (QrFormatInformation) values() QrFormatInformationValues {
	return QrFormatInformationValues{
		LevelBits:         0,
		MaskBits:          3,
		FormatDegree:      10,
		FormatGenerator:   0x537,
		FormatMask:        0x5412,
		FormatLength:      15,
		FormatLine:        8,
		VersionDegree:     12,
		VersionGenerator:  0x1F25,
		VersionMinimum:    7,
		VersionLength:     18,
		VersionAreaWidth:  3,
		VersionAreaOffset: 11,
	}
}
