package protocol

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

type Server struct {
	settings    Settings
	slots       chan struct{}
	group       sync.WaitGroup
	mutex       sync.Mutex
	connections map[net.Conn]bool
	listeners   []net.Listener
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewServer(settings Settings) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		settings:    settings,
		slots:       make(chan struct{}, settings.MaxConnections),
		connections: map[net.Conn]bool{},
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (server *Server) Serve(listener net.Listener) error {
	server.remember(listener)
	return server.acceptAll(listener)
}

func (server *Server) Shutdown(ctx context.Context) error {
	server.closeListeners()
	err := server.await(ctx, server.finished())
	server.closeAll()
	return err
}

func (server *Server) remember(listener net.Listener) {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	server.listeners = append(server.listeners, listener)
}

func (server *Server) acceptAll(listener net.Listener) error {
	for {
		connection, err := listener.Accept()
		if err != nil {
			return server.acceptError(err)
		}
		server.admit(connection)
	}
}

func (server *Server) acceptError(err error) error {
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func (server *Server) admit(connection net.Conn) {
	select {
	case server.slots <- struct{}{}:
		server.start(connection)
	default:
		server.refuse(connection)
	}
}

func (server *Server) start(connection net.Conn) {
	server.track(connection)
	server.group.Add(1)
	go server.serve(connection)
}

func (server *Server) serve(connection net.Conn) {
	defer server.finish(connection)
	NewSession(server.settings, connection, rand.Text()).Run(server.ctx)
}

func (server *Server) finish(connection net.Conn) {
	connection.Close()
	server.forget(connection)
	<-server.slots
	server.group.Done()
}

func (server *Server) refuse(connection net.Conn) {
	connection.SetDeadline(time.Now().Add(Limits{}.RefusalTimeout()))
	io.WriteString(connection, Replies{}.Busy().Wire())
	connection.Close()
}

func (server *Server) track(connection net.Conn) {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	server.connections[connection] = true
}

func (server *Server) forget(connection net.Conn) {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	delete(server.connections, connection)
}

func (server *Server) closeListeners() {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	for _, listener := range server.listeners {
		listener.Close()
	}
}

func (server *Server) finished() <-chan struct{} {
	done := make(chan struct{})
	go server.signalWhenIdle(done)
	return done
}

func (server *Server) signalWhenIdle(done chan struct{}) {
	server.group.Wait()
	close(done)
}

func (server *Server) await(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (server *Server) closeAll() {
	server.cancel()
	server.mutex.Lock()
	defer server.mutex.Unlock()
	for connection := range server.connections {
		connection.Close()
	}
}
