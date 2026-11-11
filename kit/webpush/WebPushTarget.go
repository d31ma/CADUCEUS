package webpush

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type WebPushTarget struct {
	signer    VapidSigner
	encryptor Encryptor
	policy    EndpointPolicy
	client    *http.Client
}

func NewWebPushTarget(signer VapidSigner, encryptor Encryptor, policy EndpointPolicy, client *http.Client) WebPushTarget {
	return WebPushTarget{signer: signer, encryptor: encryptor, policy: policy, client: WebPushTarget{}.withoutRedirects(client)}
}

func (target WebPushTarget) Send(subscription Subscription, message Message) (string, error) {
	request, err := target.request(subscription, message)
	if err != nil {
		return Outcomes{}.Refused(), err
	}
	return target.deliver(request)
}

func (target WebPushTarget) request(subscription Subscription, message Message) (*http.Request, error) {
	address, err := target.policy.address(subscription.Endpoint)
	if err != nil {
		return nil, err
	}
	authorization, err := target.signer.Authorization(subscription.Endpoint)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	body, err := target.encryptor.Encrypt(subscription, payload)
	if err != nil {
		return nil, err
	}
	return target.post(address, authorization, body), nil
}

func (target WebPushTarget) post(address *url.URL, authorization string, body []byte) *http.Request {
	header := http.Header{}
	header.Set("Authorization", authorization)
	header.Set("Content-Encoding", "aes128gcm")
	header.Set("Content-Type", "application/octet-stream")
	header.Set("TTL", strconv.Itoa(target.timeToLiveSeconds()))
	header.Set("Urgency", "normal")
	return &http.Request{
		Method:        http.MethodPost,
		URL:           address,
		Host:          address.Host,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func (target WebPushTarget) deliver(request *http.Request) (string, error) {
	response, err := target.client.Do(request)
	if err != nil {
		return Outcomes{}.Retry(), PushFailure{Detail: target.networkDetail(err)}
	}
	defer response.Body.Close()
	outcome := target.outcome(response.StatusCode)
	return outcome, target.failure(outcome, response)
}

func (target WebPushTarget) outcome(status int) string {
	switch {
	case status >= http.StatusOK && status < http.StatusMultipleChoices:
		return Outcomes{}.Delivered()
	case status == http.StatusNotFound || status == http.StatusGone:
		return Outcomes{}.Gone()
	case status == http.StatusTooManyRequests || status >= http.StatusInternalServerError:
		return Outcomes{}.Retry()
	}
	return Outcomes{}.Refused()
}

func (target WebPushTarget) failure(outcome string, response *http.Response) error {
	if outcome == (Outcomes{}).Delivered() || outcome == (Outcomes{}).Gone() {
		return nil
	}
	return PushFailure{Status: response.StatusCode, Detail: target.responseDetail(response.Body)}
}

func (target WebPushTarget) responseDetail(body io.Reader) string {
	detail, err := io.ReadAll(io.LimitReader(body, target.detailLimit()))
	return strings.TrimSpace(string(detail) + target.readProblem(err))
}

func (WebPushTarget) readProblem(err error) string {
	if err != nil {
		return " (the response body could not be read: " + err.Error() + ")"
	}
	return ""
}

func (WebPushTarget) networkDetail(err error) string {
	failure := &url.Error{Err: err}
	errors.As(err, &failure)
	return failure.Err.Error()
}

func (WebPushTarget) withoutRedirects(client *http.Client) *http.Client {
	copied := http.Client{}
	if client != nil {
		copied = *client
	}
	copied.CheckRedirect = WebPushTarget{}.keepRedirect
	return &copied
}

func (WebPushTarget) keepRedirect(request *http.Request, previous []*http.Request) error {
	return http.ErrUseLastResponse
}

func (WebPushTarget) timeToLiveSeconds() int {
	return 86400
}

func (WebPushTarget) detailLimit() int64 {
	return 512
}
