package mailin

type Envelope struct {
	EnvelopeFrom  string      `json:"envelopeFrom"`
	Recipients    []string    `json:"recipients"`
	RawBlobKey    string      `json:"rawBlobKey"`
	Size          int64       `json:"size"`
	RemoteAddress string      `json:"remoteAddress"`
	Helo          string      `json:"helo"`
	Tls           bool        `json:"tls"`
	Headers       Headers     `json:"headers"`
	AuthResults   AuthResults `json:"authResults"`
}
