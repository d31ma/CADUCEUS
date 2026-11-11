package smtpclient

import (
	"bufio"
	"net"
)

type tunnel struct {
	net.Conn
	reader *bufio.Reader
}

func (connection tunnel) Read(buffer []byte) (int, error) {
	return connection.reader.Read(buffer)
}
