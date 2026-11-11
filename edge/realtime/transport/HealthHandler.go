package transport

import (
	"encoding/json"
	"net/http"
)

type HealthHandler struct{}

func (HealthHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set(Headers{}.ContentType(), Headers{}.Json())
	_ = json.NewEncoder(writer).Encode(HealthStatus{Status: "ok"})
}
