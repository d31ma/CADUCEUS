package connections_test

import (
	"errors"
	"sync"
)

type FakeTransport struct {
	mutex           sync.Mutex
	successfulSends int
	heartbeatError  error
	sends           []string
	heartbeats      int
	closeCode       int
}

func (transport *FakeTransport) Send(event []byte) error {
	transport.mutex.Lock()
	defer transport.mutex.Unlock()
	transport.sends = append(transport.sends, string(event))
	if len(transport.sends) > transport.successfulSends {
		return errors.New("the connection is broken")
	}
	return nil
}

func (transport *FakeTransport) Heartbeat() error {
	transport.mutex.Lock()
	defer transport.mutex.Unlock()
	transport.heartbeats++
	return transport.heartbeatError
}

func (transport *FakeTransport) Close(code int) {
	transport.mutex.Lock()
	defer transport.mutex.Unlock()
	transport.closeCode = code
}

func (transport *FakeTransport) Sends() []string {
	transport.mutex.Lock()
	defer transport.mutex.Unlock()
	return append([]string(nil), transport.sends...)
}

func (transport *FakeTransport) LastSend() string {
	sends := transport.Sends()
	return sends[len(sends)-1]
}

func (transport *FakeTransport) Heartbeats() int {
	transport.mutex.Lock()
	defer transport.mutex.Unlock()
	return transport.heartbeats
}

func (transport *FakeTransport) CloseCode() int {
	transport.mutex.Lock()
	defer transport.mutex.Unlock()
	return transport.closeCode
}
