package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

type Problems struct {
}

func (Problems) UnknownService(name string) Problem {
	return Problem{Status: http.StatusNotFound, Code: "unknown-service", Message: "no service is named " + name}
}

func (Problems) BodyTooLarge() Problem {
	return Problem{Status: http.StatusRequestEntityTooLarge, Code: "body-too-large", Message: "the request body is too large"}
}

func (Problems) FunctionFailed() Problem {
	return Problem{Status: http.StatusBadGateway, Code: "function-error", Message: "the service failed to answer"}
}

func (Problems) UpstreamFailed() Problem {
	return Problem{Status: http.StatusBadGateway, Code: "upstream-error", Message: "the storage failed to answer"}
}

func (Problems) TooManyRequests() Problem {
	return Problem{Status: http.StatusTooManyRequests, Code: "too-many-requests", Message: "too many attempts; wait and try again"}
}

func (Problems) NotFound() Problem {
	return Problem{Status: http.StatusNotFound, Code: "not-found", Message: "nothing is at this path"}
}

func (Problems) BadPath() Problem {
	return Problem{Status: http.StatusBadRequest, Code: "bad-path", Message: "the path is not allowed"}
}

func (Problems) MethodNotAllowed() Problem {
	return Problem{Status: http.StatusMethodNotAllowed, Code: "method-not-allowed", Message: "only GET and HEAD are allowed here"}
}

func (Problems) OnlyGet() Problem {
	return Problem{Status: http.StatusMethodNotAllowed, Code: "method-not-allowed", Message: "only GET is allowed here"}
}

func (problems Problems) Reply(err error) Reply {
	var problem Problem
	if !errors.As(err, &problem) {
		slog.Error("request failed", "error", err.Error())
		problem = problems.FunctionFailed()
	}
	return problems.write(problem)
}

func (Problems) write(problem Problem) Reply {
	body, _ := json.Marshal(problem)
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	return Reply{Status: problem.Status, Header: header, Body: body}
}
