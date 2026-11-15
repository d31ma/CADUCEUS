package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

type Problems struct {
}

func (problems Problems) Unexpected() Problem {
	return Problem{Status: http.StatusBadGateway, Code: problems.functionError(), Message: "Something went wrong.", ErrorNumber: 5500}
}

func (problems Problems) FunctionFailed() Problem {
	return Problem{Status: http.StatusBadGateway, Code: problems.functionError(), Message: "The service failed to answer. Try again later.", ErrorNumber: 5501}
}

func (problems Problems) FunctionBusy() Problem {
	return Problem{Status: http.StatusTooManyRequests, Code: problems.functionBusy(), Message: "The service is busy. Try again later.", ErrorNumber: 5502}
}

func (problems Problems) FunctionThrottled() Problem {
	return Problem{Status: http.StatusTooManyRequests, Code: problems.functionBusy(), Message: "The service is busy. Try again later.", ErrorNumber: 5503}
}

func (Problems) TooManyRequests() Problem {
	return Problem{Status: http.StatusTooManyRequests, Code: "too-many-requests", Message: "Too many attempts. Wait and try again.", ErrorNumber: 5504}
}

func (Problems) UnknownService() Problem {
	return Problem{Status: http.StatusNotFound, Code: "unknown-service", Message: "No service has this name.", ErrorNumber: 5505}
}

func (Problems) BodyTooLarge() Problem {
	return Problem{Status: http.StatusRequestEntityTooLarge, Code: "body-too-large", Message: "The request body is too large.", ErrorNumber: 5506}
}

func (problems Problems) StorageFailed() Problem {
	return Problem{Status: http.StatusBadGateway, Code: problems.upstreamError(), Message: "The storage failed to answer. Try again later.", ErrorNumber: 5507}
}

func (problems Problems) MissingFile() Problem {
	return Problem{Status: http.StatusNotFound, Code: problems.notFound(), Message: "Nothing is at this path.", ErrorNumber: 5508}
}

func (problems Problems) MissingObject() Problem {
	return Problem{Status: http.StatusNotFound, Code: problems.notFound(), Message: "Nothing is at this path.", ErrorNumber: 5509}
}

func (problems Problems) NotFound() Problem {
	return Problem{Status: http.StatusNotFound, Code: problems.notFound(), Message: "Nothing is at this path.", ErrorNumber: 5510}
}

func (Problems) BadPath() Problem {
	return Problem{Status: http.StatusBadRequest, Code: "bad-path", Message: "This path is not allowed.", ErrorNumber: 5511}
}

func (problems Problems) OnlyGetAndHead() Problem {
	return Problem{Status: http.StatusMethodNotAllowed, Code: problems.methodNotAllowed(), Message: "Only GET and HEAD are allowed here.", ErrorNumber: 5512}
}

func (problems Problems) OnlyGet() Problem {
	return Problem{Status: http.StatusMethodNotAllowed, Code: problems.methodNotAllowed(), Message: "Only GET is allowed here.", ErrorNumber: 5513}
}

func (problems Problems) OnlyGetAndPut() Problem {
	return Problem{Status: http.StatusMethodNotAllowed, Code: problems.methodNotAllowed(), Message: "Only GET and PUT are allowed here.", ErrorNumber: 5514}
}

func (problems Problems) UpstreamFailed() Problem {
	return Problem{Status: http.StatusBadGateway, Code: problems.upstreamError(), Message: "The service failed to answer. Try again later.", ErrorNumber: 5515}
}

func (problems Problems) PassthroughFailed() Problem {
	return Problem{Status: http.StatusBadGateway, Code: problems.upstreamError(), Message: "Live updates are not available now. Try again later.", ErrorNumber: 5516}
}

func (problems Problems) KeycloakFailed() Problem {
	return Problem{Status: http.StatusBadGateway, Code: problems.upstreamError(), Message: "Sign-in is not available now. Try again later.", ErrorNumber: 5517}
}

func (problems Problems) InvokeFailed(status int) Problem {
	return problems.invokeProblem(status == http.StatusTooManyRequests)
}

func (problems Problems) invokeProblem(throttled bool) Problem {
	if throttled {
		return problems.FunctionThrottled()
	}
	return problems.FunctionFailed()
}

func (problems Problems) Reply(err error) Reply {
	var problem Problem
	if !errors.As(err, &problem) {
		slog.Error("request failed", "error", err.Error())
		problem = problems.Unexpected()
	}
	return problems.write(problem)
}

func (Problems) write(problem Problem) Reply {
	body, _ := json.Marshal(problem.Numbered())
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	return Reply{Status: problem.Status, Header: header, Body: body}
}

func (Problems) functionError() string {
	return "function-error"
}

func (Problems) functionBusy() string {
	return "function-busy"
}

func (Problems) upstreamError() string {
	return "upstream-error"
}

func (Problems) notFound() string {
	return "not-found"
}

func (Problems) methodNotAllowed() string {
	return "method-not-allowed"
}
