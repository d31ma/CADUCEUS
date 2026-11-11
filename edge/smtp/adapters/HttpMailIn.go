package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"mesail.dev/edge/smtp/reception"
)

type HttpMailIn struct {
	baseUrl  string
	requests AuthorizedRequests
	client   *http.Client
}

func NewHttpMailIn(baseUrl string, tokens reception.ServiceTokens, client *http.Client) HttpMailIn {
	return HttpMailIn{baseUrl: strings.TrimSuffix(baseUrl, "/"), requests: NewAuthorizedRequests(tokens), client: client}
}

func (mailIn HttpMailIn) Accept(ctx context.Context, envelope reception.InboundEnvelope) (string, error) {
	body, err := json.Marshal(envelope)
	if err != nil {
		return "", err
	}
	request, err := mailIn.requests.New(ctx, http.MethodPost, mailIn.baseUrl+"/api/v1/mail-in/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", mailIn.requests.JsonType())
	response, err := mailIn.client.Do(request)
	if err != nil {
		return "", err
	}
	defer mailIn.requests.Finish(response)
	return mailIn.readerFor(response.StatusCode)(response)
}

func (mailIn HttpMailIn) readerFor(status int) func(*http.Response) (string, error) {
	if status == http.StatusCreated {
		return mailIn.accepted
	}
	return mailIn.unexpected
}

func (mailIn HttpMailIn) accepted(response *http.Response) (string, error) {
	var answer InboundAccepted
	if err := json.NewDecoder(response.Body).Decode(&answer); err != nil {
		return "", err
	}
	return mailIn.requireId(answer.InboundId)
}

func (HttpMailIn) requireId(inboundId string) (string, error) {
	if inboundId == "" {
		return "", MissingValue{Name: "inboundId"}
	}
	return inboundId, nil
}

func (HttpMailIn) unexpected(response *http.Response) (string, error) {
	return "", UnexpectedStatus{Service: "mail-in", Status: response.StatusCode}
}
