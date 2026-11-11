package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseUrl string
	secret  string
	http    *http.Client
	retry   RetryPolicy
}

func NewClient(baseUrl string, secret string, timeout time.Duration) Client {
	return NewRetryingClient(baseUrl, secret, timeout, DefaultRetryPolicy())
}

func NewRetryingClient(baseUrl string, secret string, timeout time.Duration, retry RetryPolicy) Client {
	return Client{baseUrl: baseUrl, secret: secret, http: &http.Client{Timeout: timeout}, retry: retry}
}

func (client Client) Connect(ctx context.Context, request ConnectRequest) error {
	_, err := client.post(ctx, client.connectPath(), request)
	return err
}

func (client Client) Message(ctx context.Context, request MessageRequest) (Reply, error) {
	body, err := client.post(ctx, client.messagePath(), request)
	if err != nil {
		return Reply{}, err
	}
	reply := Reply{}
	err = json.Unmarshal(body, &reply)
	return reply, err
}

func (client Client) Disconnect(ctx context.Context, request DisconnectRequest) error {
	_, err := client.post(ctx, client.disconnectPath(), request)
	return err
}

func (client Client) post(ctx context.Context, path string, payload any) ([]byte, error) {
	body, err := client.once(ctx, path, payload)
	for _, delay := range client.retry.Delays {
		body, err = client.again(ctx, Attempt{Path: path, Payload: payload, Delay: delay}, body, err)
	}
	return body, err
}

func (client Client) again(ctx context.Context, attempt Attempt, body []byte, failure error) ([]byte, error) {
	if !client.retry.Retryable(failure) || !client.retry.Wait(ctx, attempt.Delay) {
		return body, failure
	}
	return client.once(ctx, attempt.Path, attempt.Payload)
}

func (client Client) once(ctx context.Context, path string, payload any) ([]byte, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseUrl+path, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(client.secretHeader(), client.secret)
	response, err := client.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, client.maxAnswerBytes()))
	if err != nil {
		return nil, err
	}
	return body, client.checkStatus(path, response.StatusCode)
}

func (Client) checkStatus(path string, status int) error {
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return UnexpectedStatus{Path: path, Status: status}
	}
	return nil
}

func (Client) connectPath() string    { return "/api/v1/realtime/gateway/connect" }
func (Client) messagePath() string    { return "/api/v1/realtime/gateway/message" }
func (Client) disconnectPath() string { return "/api/v1/realtime/gateway/disconnect" }
func (Client) secretHeader() string   { return "X-Realtime-Gateway" }
func (Client) maxAnswerBytes() int64  { return 65536 }
