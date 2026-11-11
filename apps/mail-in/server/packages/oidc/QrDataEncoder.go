package oidc

type QrDataEncoder struct{}

func (QrDataEncoder) byteMode() int {
	return 4
}

func (QrDataEncoder) modeBits() int {
	return 4
}

func (QrDataEncoder) terminatorBits() int {
	return 4
}

func (QrDataEncoder) padCodewords() []byte {
	return []byte{0xEC, 0x11}
}

func (encoder QrDataEncoder) encode(data []byte, version int) []byte {
	table := QrVersionTable{}
	buffer := &QrBitBuffer{}
	buffer.append(encoder.byteMode(), encoder.modeBits())
	buffer.append(len(data), table.countBits(version))
	buffer.appendBytes(data)
	capacity := table.dataCodewords(version)
	freeBits := capacity*buffer.bitsPerByte() - buffer.length()
	buffer.append(0, min(encoder.terminatorBits(), freeBits))
	buffer.append(0, buffer.paddingToByte())
	return encoder.pad(buffer.bytes(), capacity)
}

func (encoder QrDataEncoder) pad(codewords []byte, capacity int) []byte {
	padding := encoder.padCodewords()
	for index := 0; len(codewords) < capacity; index++ {
		codewords = append(codewords, padding[index%len(padding)])
	}
	return codewords
}
