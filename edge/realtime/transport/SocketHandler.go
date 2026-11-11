package transport

import (
	"net/http"

	"github.com/coder/websocket"
	"mesail.dev/edge/realtime/connections"
)

type SocketHandler struct {
	relay    connections.Relay
	settings connections.SessionSettings
}

func NewSocketHandler(relay connections.Relay, settings connections.SessionSettings) SocketHandler {
	return SocketHandler{relay: relay, settings: settings}
}

func (handler SocketHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	connection, err := websocket.Accept(writer, request, handler.acceptOptions())
	if err != nil {
		return
	}
	session, err := handler.relay.Open(connections.Kinds{}.Socket())
	handler.closeWhenRefused(session, err)
	done := make(chan struct{})
	transport := NewSocketTransport(connection, handler.settings.WriteTimeout)
	go connections.NewWriter(session, transport, handler.settings.PingInterval).Run(done)
	handler.readAll(NewFrameReader(connection), session)
	<-done
	handler.relay.Finish(session)
}

func (SocketHandler) acceptOptions() *websocket.AcceptOptions {
	return &websocket.AcceptOptions{InsecureSkipVerify: true, CompressionMode: websocket.CompressionDisabled}
}

func (SocketHandler) closeWhenRefused(session *connections.Session, err error) {
	if err != nil {
		session.Close(connections.CloseCodes{}.InternalError())
	}
}

func (handler SocketHandler) readAll(reader FrameReader, session *connections.Session) {
	for handler.readOne(reader, session) {
	}
}

func (handler SocketHandler) readOne(reader FrameReader, session *connections.Session) bool {
	body, problem := reader.Next()
	if problem != 0 {
		session.Close(problem)
		return false
	}
	handler.relay.Forward(session, body)
	return true
}
