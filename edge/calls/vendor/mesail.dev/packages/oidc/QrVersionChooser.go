package oidc

import "sort"

type QrVersionChooser struct{}

func (chooser QrVersionChooser) choose(length int) (int, error) {
	capacities := chooser.capacities()
	version := sort.SearchInts(capacities, length) + QrVersionTable{}.minimumVersion()
	return chooser.checked(version, length, capacities[len(capacities)-1])
}

func (chooser QrVersionChooser) capacities() []int {
	table := QrVersionTable{}
	result := []int{}
	for version := table.minimumVersion(); version <= table.maximumVersion(); version++ {
		result = append(result, chooser.capacity(version))
	}
	return result
}

func (QrVersionChooser) capacity(version int) int {
	table := QrVersionTable{}
	byteSize := QrBitBuffer{}.bitsPerByte()
	freeBits := table.dataCodewords(version)*byteSize - QrDataEncoder{}.modeBits() - table.countBits(version)
	return freeBits / byteSize
}

func (QrVersionChooser) checked(version int, length int, capacity int) (int, error) {
	table := QrVersionTable{}
	if version > table.maximumVersion() {
		return 0, QrTooLong{Length: length, Capacity: capacity}
	}
	return version, nil
}
