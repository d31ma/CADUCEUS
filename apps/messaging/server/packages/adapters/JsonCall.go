package adapters

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"mesail.dev/packages/core"
)

type JsonCall struct {
	client *http.Client
}

func NewJsonCall(timeout time.Duration) JsonCall {
	return JsonCall{client: &http.Client{Transport: http.DefaultTransport, Timeout: timeout}}
}

func (JsonCall) DefaultTimeout() time.Duration {
	return 30 * time.Second
}

func (call JsonCall) Send(request JsonRequest, target any) (bool, error) {
	prepared, err := call.prepared(request)
	if err != nil {
		return false, call.unavailable(err)
	}
	response, err := call.client.Do(prepared)
	if err != nil {
		return false, call.unavailable(err)
	}
	defer response.Body.Close()
	decodeFailure := json.NewDecoder(io.LimitReader(response.Body, call.maxResponseBytes())).Decode(target)
	return call.success(response.StatusCode), call.outcome(response.StatusCode, decodeFailure)
}

func (call JsonCall) prepared(request JsonRequest) (*http.Request, error) {
	body, err := call.bodyOf(request.Body)
	if err != nil {
		return nil, err
	}
	prepared, err := http.NewRequest(request.Method, request.Url, body)
	if err != nil {
		return nil, err
	}
	prepared.Header.Set("Authorization", request.Authorization)
	prepared.Header.Set("Content-Type", "application/json")
	prepared.Header.Set("Accept", "application/json")
	return prepared, nil
}

func (JsonCall) bodyOf(body any) (io.Reader, error) {
	if body == nil {
		return http.NoBody, nil
	}
	encoded, err := json.Marshal(body)
	return bytes.NewReader(encoded), err
}

func (call JsonCall) success(status int) bool {
	return status/call.classSize() == call.successClass()
}

func (call JsonCall) outcome(status int, decodeFailure error) error {
	switch {
	case status == http.StatusNotFound:
		return nil
	case !call.success(status):
		return &core.Unavailable{Message: "the service answered " + strconv.Itoa(status)}
	case decodeFailure != nil:
		return call.unavailable(decodeFailure)
	default:
		return nil
	}
}

func (JsonCall) unavailable(failure error) error {
	return &core.Unavailable{Message: "the service is not available", Cause: failure}
}

func (JsonCall) maxResponseBytes() int64 {
	return 8 << 20
}

func (JsonCall) classSize() int {
	return 100
}

func (JsonCall) successClass() int {
	return 2
}
