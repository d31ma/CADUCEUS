package transport

import (
	"context"
	"time"

	"github.com/coder/websocket"
)

type SocketTransport struct {
	connection   *websocket.Conn
	writeTimeout time.Duration
}

func NewSocketTransport(connection *websocket.Conn, writeTimeout time.Duration) SocketTransport {
	return SocketTransport{connection: connection, writeTimeout: writeTimeout}
}

func (transport SocketTransport) Send(event []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), transport.writeTimeout)
	defer cancel()
	return transport.connection.Write(ctx, websocket.MessageText, event)
}

func (transport SocketTransport) Heartbeat() error {
	go transport.ping()
	return nil
}

func (transport SocketTransport) Close(code int) {
	_ = transport.connection.Close(websocket.StatusCode(code), "")
}

func (transport SocketTransport) ping() {
	ctx, cancel := context.WithTimeout(context.Background(), transport.writeTimeout)
	defer cancel()
	transport.dropWhenSilent(transport.connection.Ping(ctx))
}

func (transport SocketTransport) dropWhenSilent(err error) {
	if err != nil {
		_ = transport.connection.CloseNow()
	}
}
