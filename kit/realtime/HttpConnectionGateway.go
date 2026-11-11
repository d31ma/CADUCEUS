package realtime

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type HttpConnectionGateway struct {
	BaseUrl string
	Secret  string
	Client  *http.Client
}

func (gateway HttpConnectionGateway) Push(connectionId string, event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return gateway.call(http.MethodPost, gateway.address(connectionId), bytes.NewReader(body), connectionId)
}

func (gateway HttpConnectionGateway) Close(connectionId string, code int) error {
	address := gateway.address(connectionId) + "?code=" + strconv.Itoa(code)
	return gateway.call(http.MethodDelete, address, http.NoBody, connectionId)
}

func (gateway HttpConnectionGateway) call(method string, address string, body io.Reader, connectionId string) error {
	request, err := http.NewRequest(method, address, body)
	if err != nil {
		return Unavailable{Operation: gateway.operation(method)}
	}
	request.Header.Set(gateway.secretHeader(), gateway.Secret)
	request.Header.Set("Content-Type", "application/json")
	response, err := gateway.Client.Do(request)
	if err != nil {
		return Unavailable{Operation: gateway.operation(method)}
	}
	defer response.Body.Close()
	io.Copy(io.Discard, response.Body)
	return gateway.outcome(response.StatusCode, method, connectionId)
}

func (gateway HttpConnectionGateway) outcome(status int, method string, connectionId string) error {
	switch {
	case status == http.StatusGone:
		return Gone{ConnectionId: connectionId}
	case status < http.StatusOK || status >= http.StatusMultipleChoices:
		return Unavailable{Operation: gateway.operation(method)}
	}
	return nil
}

func (gateway HttpConnectionGateway) address(connectionId string) string {
	return gateway.BaseUrl + "/@connections/" + url.PathEscape(connectionId)
}

func (HttpConnectionGateway) operation(method string) string {
	return "gateway " + method
}

func (HttpConnectionGateway) secretHeader() string {
	return "X-Realtime-Gateway"
}
