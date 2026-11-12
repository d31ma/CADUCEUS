//go:build blackbox

package blackbox

import (
	"bytes"
	"encoding/binary"
)

type CborWriter struct {
	buffer bytes.Buffer
}

func (writer *CborWriter) Bytes() []byte {
	return writer.buffer.Bytes()
}

func (writer *CborWriter) MapHeader(size int) {
	writer.head(5, uint64(size))
}

func (writer *CborWriter) Integer(value int) {
	if value < 0 {
		writer.head(1, uint64(-1-value))
		return
	}
	writer.head(0, uint64(value))
}

func (writer *CborWriter) ByteString(value []byte) {
	writer.head(2, uint64(len(value)))
	writer.buffer.Write(value)
}

func (writer *CborWriter) Text(value string) {
	writer.head(3, uint64(len(value)))
	writer.buffer.WriteString(value)
}

func (writer *CborWriter) head(major byte, length uint64) {
	kind := major << 5
	switch {
	case length < 24:
		writer.buffer.WriteByte(kind | byte(length))
	case length < 1<<8:
		writer.buffer.WriteByte(kind | 24)
		writer.buffer.WriteByte(byte(length))
	case length < 1<<16:
		writer.buffer.WriteByte(kind | 25)
		_ = binary.Write(&writer.buffer, binary.BigEndian, uint16(length))
	default:
		writer.buffer.WriteByte(kind | 26)
		_ = binary.Write(&writer.buffer, binary.BigEndian, uint32(length))
	}
}
