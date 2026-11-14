package transport

import (
	"encoding/json"
	"net/http"

	"mesail.dev/edge/calls/failures"
)

type Replies struct{}

func (replies Replies) Json(writer http.ResponseWriter, value any, err error) {
	replies.valueOrProblem(writer, value, err)
}

func (replies Replies) Empty(writer http.ResponseWriter, err error) {
	replies.emptyOrProblem(writer, err)
}

func (replies Replies) valueOrProblem(writer http.ResponseWriter, value any, err error) {
	if err != nil {
		replies.problem(writer, err)
		return
	}
	replies.write(writer, http.StatusOK, value)
}

func (replies Replies) emptyOrProblem(writer http.ResponseWriter, err error) {
	if err != nil {
		replies.problem(writer, err)
		return
	}
	replies.headers(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (replies Replies) problem(writer http.ResponseWriter, err error) {
	problem := failures.Problems{}.From(err)
	replies.write(writer, problem.Status, ApiError{Code: problem.Code, Message: problem.Message(), Field: problem.Field})
}

func (replies Replies) write(writer http.ResponseWriter, status int, value any) {
	replies.headers(writer)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func (Replies) headers(writer http.ResponseWriter) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
}
