package reception

import (
	"net"
	"time"
)

type AuthInput struct {
	RemoteIp   net.IP
	Helo       string
	MailFrom   string
	Raw        []byte
	ReceivedAt time.Time
}
