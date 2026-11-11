package main

import "net/http"

type Reply struct {
	Status int
	Header http.Header
	Body   []byte
}

func (reply Reply) Send(writer http.ResponseWriter) {
	reply.copyHeader(writer.Header())
	writer.WriteHeader(reply.Status)
	_, _ = writer.Write(reply.Body)
}

func (reply Reply) copyHeader(target http.Header) {
	for name, values := range reply.Header {
		target[name] = values
	}
}
