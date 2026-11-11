package connections

import "time"

type Writer struct {
	session   *Session
	transport Transport
	interval  time.Duration
}

func NewWriter(session *Session, transport Transport, interval time.Duration) Writer {
	return Writer{session: session, transport: transport, interval: interval}
}

func (writer Writer) Run(done chan<- struct{}) {
	ticker := time.NewTicker(writer.interval)
	writer.loop(ticker.C)
	ticker.Stop()
	writer.session.Close(CloseCodes{}.Broken())
	writer.sayFarewell(writer.session.Farewell())
	writer.transport.Close(writer.session.CloseCode())
	close(done)
}

func (writer Writer) loop(ticks <-chan time.Time) {
	for writer.step(ticks) == nil {
	}
}

func (writer Writer) step(ticks <-chan time.Time) error {
	select {
	case <-writer.session.Closing():
		return SessionClosed{}
	case event := <-writer.session.Events():
		return writer.deliver(event)
	case <-ticks:
		return writer.transport.Heartbeat()
	}
}

func (writer Writer) deliver(event []byte) error {
	select {
	case <-writer.session.Closing():
		return SessionClosed{}
	default:
		return writer.transport.Send(event)
	}
}

func (writer Writer) sayFarewell(farewell []byte) {
	if farewell == nil {
		return
	}
	_ = writer.transport.Send(farewell)
}
