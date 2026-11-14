package transport

import (
	"net/http"

	"mesail.dev/edge/calls/desk"
)

type CandidatesHandler struct {
	calls desk.CallDesk
}

func (handler CandidatesHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	Replies{}.Empty(writer, handler.calls.AddCandidates(RequestReader{}.From(request, handler.maximumBytes())))
}

func (CandidatesHandler) maximumBytes() int64 {
	return 16 << 10
}
