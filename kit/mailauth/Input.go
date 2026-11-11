package mailauth

import (
	"net"
	"time"
)

type Input struct {
	RemoteIp   net.IP
	Helo       string
	MailFrom   string
	Raw        []byte
	ReceivedAt time.Time
}
