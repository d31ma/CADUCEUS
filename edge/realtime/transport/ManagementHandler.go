package transport

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"mesail.dev/edge/realtime/connections"
)

type ManagementHandler struct {
	hub *connections.Hub
}

func NewManagementHandler(hub *connections.Hub) ManagementHandler {
	return ManagementHandler{hub: hub}
}

func (handler ManagementHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(handler.pattern(http.MethodPost), handler.Push)
	mux.HandleFunc(handler.pattern(http.MethodDelete), handler.Close)
	return mux
}

func (handler ManagementHandler) Push(writer http.ResponseWriter, request *http.Request) {
	event, err := handler.readEvent(writer, request)
	if err != nil {
		handler.refuse(writer, err)
		return
	}
	handler.answer(writer, handler.deliver(handler.connectionId(request), event), http.StatusOK)
}

func (handler ManagementHandler) Close(writer http.ResponseWriter, request *http.Request) {
	code, err := CloseCodeQuery{}.Parse(request.URL.Query().Get("code"))
	if err != nil {
		handler.refuse(writer, err)
		return
	}
	handler.answer(writer, handler.closeSession(handler.connectionId(request), code), http.StatusNoContent)
}

func (handler ManagementHandler) readEvent(writer http.ResponseWriter, request *http.Request) ([]byte, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, handler.maxEventBytes()))
	if err != nil {
		return nil, handler.readProblem(err)
	}
	compact := bytes.Buffer{}
	err = json.Compact(&compact, raw)
	if err != nil {
		return nil, RequestProblem{Status: http.StatusBadRequest, Text: "event must be JSON"}
	}
	return compact.Bytes(), nil
}

func (ManagementHandler) readProblem(err error) RequestProblem {
	tooLarge := &http.MaxBytesError{}
	if errors.As(err, &tooLarge) {
		return RequestProblem{Status: http.StatusRequestEntityTooLarge, Text: "event too large"}
	}
	return RequestProblem{Status: http.StatusBadRequest, Text: "event could not be read"}
}

func (ManagementHandler) refuse(writer http.ResponseWriter, err error) {
	problem := RequestProblem{Status: http.StatusBadRequest, Text: "bad request"}
	errors.As(err, &problem)
	http.Error(writer, problem.Text, problem.Status)
}

func (handler ManagementHandler) deliver(connectionId string, event []byte) bool {
	session, found := handler.hub.Find(connectionId)
	if !found {
		return false
	}
	return session.Push(event)
}

func (handler ManagementHandler) closeSession(connectionId string, code int) bool {
	session, found := handler.hub.Find(connectionId)
	if !found {
		return false
	}
	session.Close(code)
	return true
}

func (ManagementHandler) answer(writer http.ResponseWriter, done bool, status int) {
	if !done {
		http.Error(writer, "gone", http.StatusGone)
		return
	}
	writer.WriteHeader(status)
}

func (handler ManagementHandler) connectionId(request *http.Request) string {
	return request.PathValue(handler.idName())
}

func (handler ManagementHandler) pattern(method string) string {
	return method + " /@connections/{" + handler.idName() + "}"
}

func (ManagementHandler) idName() string { return "connectionId" }

func (ManagementHandler) maxEventBytes() int64 { return 65536 }
