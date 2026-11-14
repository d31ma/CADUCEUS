package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"mesail.dev/edge/calls/desk"
	"mesail.dev/edge/calls/failures"
)

type RequestReader struct{}

func (reader RequestReader) From(request *http.Request, limit int64) desk.Request {
	return desk.Request{
		Token:  request.Header.Get("X-Mesail-Join"),
		CallId: request.PathValue("callId"),
		Decode: func(into any) error { return reader.decoded(request, limit, into) },
	}
}

func (reader RequestReader) decoded(request *http.Request, limit int64, into any) error {
	err := json.NewDecoder(http.MaxBytesReader(nil, request.Body, limit)).Decode(into)
	return reader.problem(err)
}

func (RequestReader) problem(err error) error {
	tooLarge := &http.MaxBytesError{}
	switch {
	case err == nil:
		return nil
	case errors.As(err, &tooLarge):
		return failures.Problems{}.TooLarge()
	}
	return failures.Problems{}.InvalidJson()
}
