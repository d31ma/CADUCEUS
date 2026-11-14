package transport

import (
	"net/http"

	"mesail.dev/edge/calls/desk"
)

type OfferHandler struct {
	calls desk.CallDesk
}

func (handler OfferHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	answer, err := handler.calls.Offer(RequestReader{}.From(request, handler.maximumBytes()))
	Replies{}.Json(writer, answer, err)
}

func (OfferHandler) maximumBytes() int64 {
	return 256 << 10
}
