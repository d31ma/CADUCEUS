package protocol

import (
	"crypto/tls"
	"log"

	"mesail.dev/edge/smtp/reception"
)

type Settings struct {
	Hostname       string
	MaxSize        int64
	MaxConnections int
	Tls            *tls.Config
	Directory      reception.AddressDirectory
	Delivery       Delivery
	Log            *log.Logger
}
