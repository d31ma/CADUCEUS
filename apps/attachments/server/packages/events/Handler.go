package events

type Handler interface {
	Handle(envelope Envelope) error
}
