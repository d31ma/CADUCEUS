package reception

import "time"

type Transaction struct {
	EnvelopeFrom  string
	Recipients    []Recipient
	Raw           []byte
	RemoteAddress string
	Helo          string
	Tls           bool
	ReceivedAt    time.Time
}
