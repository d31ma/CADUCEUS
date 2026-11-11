package main

import "net/http"

type ReadyHandler struct {
}

func (ReadyHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	Reply{Status: http.StatusOK, Header: header, Body: []byte(`{"status":"ok"}`)}.Send(writer)
}
