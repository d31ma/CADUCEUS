package transport

import (
	"net/http"
	"strconv"

	"mesail.dev/edge/realtime/connections"
)

type CloseCodeQuery struct{}

func (query CloseCodeQuery) Parse(text string) (int, error) {
	code, err := strconv.Atoi(query.orDefault(text))
	if err != nil {
		return 0, query.problem()
	}
	return code, query.check(code)
}

func (CloseCodeQuery) orDefault(text string) string {
	if text == "" {
		return strconv.Itoa(connections.CloseCodes{}.Normal())
	}
	return text
}

func (query CloseCodeQuery) check(code int) error {
	codes := connections.CloseCodes{}
	if code == codes.Normal() || (code >= codes.FirstPrivate() && code <= codes.LastPrivate()) {
		return nil
	}
	return query.problem()
}

func (CloseCodeQuery) problem() RequestProblem {
	return RequestProblem{Status: http.StatusBadRequest, Text: "code must be 1000 or 4000-4999"}
}
