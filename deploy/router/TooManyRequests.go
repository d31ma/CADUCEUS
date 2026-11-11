package main

import "net/http"

type TooManyRequests struct {
	RetryAfter string
}

func (refusal TooManyRequests) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	reply := Problems{}.Reply(Problems{}.TooManyRequests())
	reply.Header.Set("Retry-After", refusal.RetryAfter)
	reply.Send(writer)
}
