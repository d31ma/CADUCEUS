package main

import "net/http"

type StatusRecorder struct {
	http.ResponseWriter
	Status int
}

func (recorder *StatusRecorder) WriteHeader(status int) {
	recorder.Status = status
	recorder.ResponseWriter.WriteHeader(status)
}
