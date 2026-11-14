package transport

import "net/http"

type HealthHandler struct{}

func (HealthHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	Replies{}.Json(writer, Health{Service: "calls", Status: "ok"}, nil)
}
