package main

import "net/http"

type NotFoundHandler struct {
}

func (NotFoundHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	Problems{}.Reply(Problems{}.NotFound()).Send(writer)
}
