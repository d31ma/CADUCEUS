package main

import (
	"log/slog"
	"net/http"
)

type ProxyFailure struct {
	Problem Problem
}

func (failure ProxyFailure) Answer(writer http.ResponseWriter, request *http.Request, err error) {
	slog.Error("proxy failed", "number", failure.Problem.Number(), "error", err.Error())
	Problems{}.Reply(failure.Problem).Send(writer)
}
