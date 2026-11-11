package transport

import (
	"context"
	"encoding/json"
	"net/http"

	"mesail.dev/edge/realtime/connections"
)

type StreamHandler struct {
	relay    connections.Relay
	settings connections.SessionSettings
}

func NewStreamHandler(relay connections.Relay, settings connections.SessionSettings) StreamHandler {
	return StreamHandler{relay: relay, settings: settings}
}

func (handler StreamHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ticket, err := handler.ticket(request)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	session, err := handler.relay.Open(connections.Kinds{}.Stream())
	if err != nil {
		handler.refuse(writer, session)
		return
	}
	handler.serve(request.Context(), writer, session, ticket)
}

func (handler StreamHandler) serve(ctx context.Context, writer http.ResponseWriter, session *connections.Session, ticket []byte) {
	transport := NewStreamTransport(writer, handler.settings.WriteTimeout)
	err := transport.Open()
	if err != nil {
		session.Close(connections.CloseCodes{}.Broken())
	}
	done := make(chan struct{})
	go connections.NewWriter(session, transport, handler.settings.KeepAliveInterval).Run(done)
	handler.relay.Forward(session, string(ticket))
	handler.watch(ctx, session)
	<-done
	handler.relay.Finish(session)
}

func (handler StreamHandler) ticket(request *http.Request) ([]byte, error) {
	value := request.URL.Query().Get(handler.ticketName())
	if value == "" {
		return nil, RequestProblem{Status: http.StatusBadRequest, Text: "ticket required"}
	}
	return json.Marshal(TicketMessage{Type: handler.ticketName(), Ticket: value})
}

func (handler StreamHandler) refuse(writer http.ResponseWriter, session *connections.Session) {
	session.Close(connections.CloseCodes{}.InternalError())
	handler.relay.Finish(session)
	http.Error(writer, "realtime backend unavailable", http.StatusServiceUnavailable)
}

func (StreamHandler) watch(ctx context.Context, session *connections.Session) {
	select {
	case <-ctx.Done():
		session.Close(connections.CloseCodes{}.Broken())
	case <-session.Closing():
	}
}

func (StreamHandler) ticketName() string { return "ticket" }
