package main

import "net/http"

type BlobsHandler struct {
	Next http.Handler
}

func (handler BlobsHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	handler.choose(request.Method).ServeHTTP(writer, request)
}

func (handler BlobsHandler) choose(method string) http.Handler {
	if method != http.MethodGet && method != http.MethodPut {
		return ProblemHandler{Err: Problems{}.OnlyGetAndPut()}
	}
	return handler.Next
}
