package oidc

type QrBlockInterleaver struct{}

func (interleaver QrBlockInterleaver) codewords(data []byte, version int) []byte {
	table := QrVersionTable{}
	blocks := interleaver.split(data, table.blockCount(version))
	corrections := interleaver.corrections(blocks, table.correctionCodewordsPerBlock(version))
	return append(interleaver.interleave(blocks), interleaver.interleave(corrections)...)
}

func (interleaver QrBlockInterleaver) split(data []byte, count int) [][]byte {
	shortLength := len(data) / count
	shortCount := count - len(data)%count
	result := [][]byte{}
	offset := 0
	for index := range count {
		length := interleaver.blockLength(index, shortCount, shortLength)
		result = append(result, data[offset:offset+length])
		offset += length
	}
	return result
}

func (QrBlockInterleaver) blockLength(index int, shortCount int, shortLength int) int {
	if index < shortCount {
		return shortLength
	}
	return shortLength + 1
}

func (QrBlockInterleaver) corrections(blocks [][]byte, degree int) [][]byte {
	polynomial := QrPolynomial{}
	result := [][]byte{}
	for _, block := range blocks {
		result = append(result, polynomial.remainder(block, degree))
	}
	return result
}

func (interleaver QrBlockInterleaver) interleave(blocks [][]byte) []byte {
	count := len(blocks)
	result := []byte{}
	for index := range interleaver.longest(blocks) * count {
		result = interleaver.appendIfPresent(result, blocks[index%count], index/count)
	}
	return result
}

func (QrBlockInterleaver) longest(blocks [][]byte) int {
	result := 0
	for _, block := range blocks {
		result = max(result, len(block))
	}
	return result
}

func (QrBlockInterleaver) appendIfPresent(result []byte, block []byte, position int) []byte {
	if position >= len(block) {
		return result
	}
	return append(result, block[position])
}
