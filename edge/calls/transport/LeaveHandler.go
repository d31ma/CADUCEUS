package transport

import (
	"net/http"

	"mesail.dev/edge/calls/desk"
)

type LeaveHandler struct {
	calls desk.CallDesk
}

func (handler LeaveHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	Replies{}.Empty(writer, handler.calls.Leave(RequestReader{}.From(request, handler.maximumBytes())))
}

func (LeaveHandler) maximumBytes() int64 {
	return 1 << 10
}
