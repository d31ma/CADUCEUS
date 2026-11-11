package main

import (
	"io"
	"net/http"
)

type BodyReader struct {
	Limit int64
}

func (reader BodyReader) Read(request *http.Request) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(request.Body, reader.Limit+1))
	if err != nil {
		return nil, err
	}
	return reader.checked(body)
}

func (reader BodyReader) checked(body []byte) ([]byte, error) {
	if int64(len(body)) > reader.Limit {
		return nil, Problems{}.BodyTooLarge()
	}
	return body, nil
}
