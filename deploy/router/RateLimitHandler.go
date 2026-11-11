package main

import (
	"net"
	"net/http"
	"slices"
	"strconv"
)

type RateLimitHandler struct {
	Limited []string
	Limiter RateLimiter
	Next    http.Handler
}

func (handler RateLimitHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	handler.choose(request).ServeHTTP(writer, request)
}

func (handler RateLimitHandler) choose(request *http.Request) http.Handler {
	if handler.refused(request) {
		return TooManyRequests{RetryAfter: strconv.Itoa(int(handler.Limiter.Window.Seconds()))}
	}
	return handler.Next
}

func (handler RateLimitHandler) refused(request *http.Request) bool {
	limited := slices.Contains(handler.Limited, request.Method+" "+request.URL.Path)
	return limited && !handler.Limiter.Allow(handler.client(request.RemoteAddr))
}

func (RateLimitHandler) client(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	return host
}
