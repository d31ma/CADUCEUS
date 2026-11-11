package sink

import (
	"net"
	"sync"
)

type Sink struct {
	listener    net.Listener
	options     Options
	mutex       sync.Mutex
	messages    []Received
	connections map[net.Conn]bool
	group       sync.WaitGroup
	closed      bool
}

func Start(options Options) (*Sink, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	sink := &Sink{listener: listener, options: options, connections: map[net.Conn]bool{}}
	sink.group.Add(1)
	go sink.acceptLoop()
	return sink, nil
}

func (sink *Sink) Address() string {
	return sink.listener.Addr().String()
}

func (sink *Sink) Port() int {
	return sink.listener.Addr().(*net.TCPAddr).Port
}

func (sink *Sink) Messages() []Received {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	return append([]Received(nil), sink.messages...)
}

func (sink *Sink) Close() error {
	err := sink.listener.Close()
	sink.markClosed()
	sink.closeConnections()
	sink.group.Wait()
	return err
}

func (sink *Sink) acceptLoop() {
	defer sink.group.Done()
	for {
		network, err := sink.listener.Accept()
		if err != nil {
			return
		}
		sink.serve(network)
	}
}

func (sink *Sink) serve(network net.Conn) {
	sink.track(network)
	sink.group.Add(1)
	go func() {
		defer sink.group.Done()
		newConnection(sink, network).run()
		sink.untrack(network)
	}()
}

func (sink *Sink) track(network net.Conn) {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	sink.connections[network] = true
	if sink.closed {
		network.Close()
	}
}

func (sink *Sink) markClosed() {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	sink.closed = true
}

func (sink *Sink) untrack(network net.Conn) {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	delete(sink.connections, network)
}

func (sink *Sink) closeConnections() {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	for network := range sink.connections {
		network.Close()
	}
}

func (sink *Sink) store(received Received) {
	sink.mutex.Lock()
	defer sink.mutex.Unlock()
	sink.messages = append(sink.messages, received)
}
