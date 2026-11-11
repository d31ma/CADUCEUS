package oidc

type QrBitBuffer struct {
	bits []int
}

func (QrBitBuffer) bitsPerByte() int {
	return 8
}

func (buffer *QrBitBuffer) append(value int, count int) {
	for shift := count - 1; shift >= 0; shift-- {
		buffer.bits = append(buffer.bits, (value>>shift)&1)
	}
}

func (buffer *QrBitBuffer) appendBytes(data []byte) {
	for _, value := range data {
		buffer.append(int(value), buffer.bitsPerByte())
	}
}

func (buffer *QrBitBuffer) length() int {
	return len(buffer.bits)
}

func (buffer *QrBitBuffer) paddingToByte() int {
	byteSize := buffer.bitsPerByte()
	return (byteSize - buffer.length()%byteSize) % byteSize
}

func (buffer *QrBitBuffer) bytes() []byte {
	byteSize := buffer.bitsPerByte()
	result := make([]byte, (buffer.length()+byteSize-1)/byteSize)
	for index, bit := range buffer.bits {
		result[index/byteSize] |= byte(bit << (byteSize - 1 - index%byteSize))
	}
	return result
}
