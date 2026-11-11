package mime

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net/textproto"
)

type headerBlock struct {
	header textproto.MIMEHeader
	body   []byte
	size   int
}

func (block headerBlock) read(raw []byte) (headerBlock, error) {
	source := bytes.NewReader(raw)
	buffered := bufio.NewReader(source)
	header, err := textproto.NewReader(buffered).ReadMIMEHeader()
	size := len(raw) - source.Len() - buffered.Buffered()
	return headerBlock{header: header, body: raw[size:], size: size}, block.readable(header, err)
}

func (headerBlock) readable(header textproto.MIMEHeader, err error) error {
	if err == nil || (errors.Is(err, io.EOF) && len(header) > 0) {
		return nil
	}
	return Malformed{}
}
