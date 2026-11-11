package sink

import "crypto/tls"

type Options struct {
	Tls             *tls.Config
	ImplicitTls     bool
	Username        string
	Password        string
	RejectRecipient func(address string) (int, string)
	MaxSize         int
}
