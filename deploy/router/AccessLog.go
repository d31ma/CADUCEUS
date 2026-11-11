package main

import (
	"log/slog"
	"net/http"
	"time"
)

type AccessLog struct {
	Next http.Handler
}

func (log AccessLog) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	started := time.Now()
	recorder := &StatusRecorder{ResponseWriter: writer, Status: http.StatusOK}
	log.Next.ServeHTTP(recorder, request)
	slog.Info("request", "method", request.Method, "path", request.URL.Path, "status", recorder.Status,
		"milliseconds", time.Since(started).Milliseconds())
}
