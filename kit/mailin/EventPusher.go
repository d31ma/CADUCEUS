package mailin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

type EventPusher struct {
	Tokens outbox.DomainTokenSource
	Client *http.Client
}

func (pusher EventPusher) Push(subscriber outbox.Subscriber, envelope events.Envelope) error {
	request, err := pusher.request(subscriber, envelope)
	if err != nil {
		return err
	}
	response, err := pusher.Client.Do(request)
	if err != nil {
		return outbox.DeliveryFailure{Subscriber: subscriber.Name, Detail: err.Error()}
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	return pusher.answer(subscriber, response)
}

func (pusher EventPusher) request(subscriber outbox.Subscriber, envelope events.Envelope) (*http.Request, error) {
	token, err := pusher.Tokens.TokenFor(envelope.DomainId)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodPost, subscriber.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", DomainTokens{}.bearerPrefix()+token)
	request.Header.Set("Content-Type", "application/json")
	return request, nil
}

func (pusher EventPusher) answer(subscriber outbox.Subscriber, response *http.Response) error {
	if pusher.statusClass(response.StatusCode) == pusher.successClass() {
		return nil
	}
	return outbox.DeliveryFailure{Subscriber: subscriber.Name, Status: response.StatusCode, Permanent: pusher.permanent(response.StatusCode), Detail: response.Status}
}

func (pusher EventPusher) permanent(status int) bool {
	refused := pusher.statusClass(status) == pusher.clientErrorClass()
	return refused && status != http.StatusRequestTimeout && status != http.StatusTooManyRequests
}

func (pusher EventPusher) statusClass(status int) int {
	return status / pusher.classSize()
}

func (EventPusher) classSize() int {
	return 100
}

func (EventPusher) successClass() int {
	return 2
}

func (EventPusher) clientErrorClass() int {
	return 4
}
