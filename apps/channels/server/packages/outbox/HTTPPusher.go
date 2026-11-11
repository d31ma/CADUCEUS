package outbox

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/transport"
)

type HTTPPusher struct {
	tokens DomainTokenSource
	client *http.Client
}

func NewHTTPPusher(tokens TokenSource, timeout time.Duration) HTTPPusher {
	return NewDomainHTTPPusher(sameToken{source: tokens}, timeout)
}

func NewDomainHTTPPusher(tokens DomainTokenSource, timeout time.Duration) HTTPPusher {
	return HTTPPusher{tokens: tokens, client: transport.SingleUseTransport{}.Client(timeout)}
}

func (HTTPPusher) DefaultTimeout() time.Duration {
	return 10 * time.Second
}

func (pusher HTTPPusher) Push(subscriber Subscriber, envelope events.Envelope) error {
	token, err := pusher.tokens.TokenFor(envelope.DomainId)
	if err != nil {
		return err
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, subscriber.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := pusher.client.Do(request)
	if err != nil {
		return DeliveryFailure{Subscriber: subscriber.Name, Detail: err.Error()}
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	return pusher.answer(subscriber, response)
}

func (pusher HTTPPusher) answer(subscriber Subscriber, response *http.Response) error {
	if pusher.statusClass(response.StatusCode) == pusher.successClass() {
		return nil
	}
	return DeliveryFailure{Subscriber: subscriber.Name, Status: response.StatusCode, Permanent: pusher.permanent(response.StatusCode), Detail: response.Status}
}

func (pusher HTTPPusher) permanent(status int) bool {
	refused := pusher.statusClass(status) == pusher.clientErrorClass()
	return refused && status != http.StatusRequestTimeout && status != http.StatusTooManyRequests
}

func (pusher HTTPPusher) statusClass(status int) int {
	return status / pusher.classSize()
}

func (HTTPPusher) classSize() int {
	return 100
}

func (HTTPPusher) successClass() int {
	return 2
}

func (HTTPPusher) clientErrorClass() int {
	return 4
}
