package main

import "net/http"

type ProblemHandler struct {
	Err error
}

func (handler ProblemHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	Problems{}.Reply(handler.Err).Send(writer)
}
