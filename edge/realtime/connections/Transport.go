package connections

type Transport interface {
	Send(event []byte) error
	Heartbeat() error
	Close(code int)
}
