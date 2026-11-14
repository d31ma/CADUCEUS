package realtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type ServiceCall struct {
	BaseUrl string
	Tokens  ServiceTokenSource
	Client  *http.Client
}

func (call ServiceCall) Post(domainId string, path string, body any, answer any) error {
	request, err := call.request(domainId, path, body)
	if err != nil {
		return err
	}
	response, err := call.Client.Do(request)
	if err != nil {
		return Unavailable{Operation: path + ": " + err.Error()}
	}
	defer response.Body.Close()
	return call.read(path, response, answer)
}

func (call ServiceCall) request(domainId string, path string, body any) (*http.Request, error) {
	authorization, tokenErr := call.authorization(domainId)
	encoded, encodeErr := json.Marshal(body)
	request, requestErr := http.NewRequest(http.MethodPost, strings.TrimSuffix(call.BaseUrl, "/")+path, bytes.NewReader(encoded))
	err := errors.Join(tokenErr, encodeErr, requestErr)
	if err != nil {
		return nil, Unavailable{Operation: path + ": " + err.Error()}
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", "application/json")
	return request, nil
}

func (call ServiceCall) authorization(domainId string) (string, error) {
	if call.Tokens == nil {
		return "", Unavailable{Operation: "no service tokens"}
	}
	return call.Tokens.Authorization(domainId)
}

func (call ServiceCall) read(path string, response *http.Response, answer any) error {
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		io.Copy(io.Discard, response.Body)
		return Unavailable{Operation: path + ": status " + strconv.Itoa(response.StatusCode)}
	}
	return call.decoded(path, response.Body, answer)
}

func (ServiceCall) decoded(path string, body io.Reader, answer any) error {
	if answer == nil {
		_, err := io.Copy(io.Discard, body)
		return err
	}
	if json.NewDecoder(body).Decode(answer) != nil {
		return Unavailable{Operation: path + ": unreadable answer"}
	}
	return nil
}
