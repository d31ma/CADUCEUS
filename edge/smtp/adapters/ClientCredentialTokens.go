package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type ClientCredentialTokens struct {
	settings TokenSettings
	client   *http.Client
	now      func() time.Time
	mutex    sync.Mutex
	token    string
	expiry   time.Time
}

func NewClientCredentialTokens(settings TokenSettings, client *http.Client, now func() time.Time) *ClientCredentialTokens {
	return &ClientCredentialTokens{settings: settings, client: client, now: now}
}

func (tokens *ClientCredentialTokens) Token(ctx context.Context) (string, error) {
	tokens.mutex.Lock()
	defer tokens.mutex.Unlock()
	return tokens.sourceFor(tokens.now())(ctx)
}

func (tokens *ClientCredentialTokens) sourceFor(now time.Time) func(context.Context) (string, error) {
	if tokens.token != "" && now.Before(tokens.expiry) {
		return tokens.cachedToken
	}
	return tokens.fetch
}

func (tokens *ClientCredentialTokens) cachedToken(_ context.Context) (string, error) {
	return tokens.token, nil
}

func (tokens *ClientCredentialTokens) fetch(ctx context.Context) (string, error) {
	secret, err := os.ReadFile(tokens.settings.SecretFile)
	if err != nil {
		return "", err
	}
	answer, err := tokens.request(ctx, strings.TrimSpace(string(secret)))
	if err != nil {
		return "", err
	}
	tokens.remember(answer)
	return answer.AccessToken, nil
}

func (tokens *ClientCredentialTokens) request(ctx context.Context, secret string) (TokenAnswer, error) {
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {tokens.settings.ClientId}, "client_secret": {secret}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokens.settings.Url, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenAnswer{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := tokens.client.Do(request)
	if err != nil {
		return TokenAnswer{}, err
	}
	defer AuthorizedRequests{}.Finish(response)
	return tokens.readerFor(response.StatusCode)(response)
}

func (tokens *ClientCredentialTokens) readerFor(status int) func(*http.Response) (TokenAnswer, error) {
	if status == http.StatusOK {
		return tokens.decode
	}
	return tokens.unexpected
}

func (tokens *ClientCredentialTokens) decode(response *http.Response) (TokenAnswer, error) {
	var answer TokenAnswer
	if err := json.NewDecoder(response.Body).Decode(&answer); err != nil {
		return TokenAnswer{}, err
	}
	return tokens.requireToken(answer)
}

func (*ClientCredentialTokens) requireToken(answer TokenAnswer) (TokenAnswer, error) {
	if answer.AccessToken == "" {
		return TokenAnswer{}, MissingValue{Name: "access_token"}
	}
	return answer, nil
}

func (*ClientCredentialTokens) unexpected(response *http.Response) (TokenAnswer, error) {
	return TokenAnswer{}, UnexpectedStatus{Service: "token service", Status: response.StatusCode}
}

func (tokens *ClientCredentialTokens) remember(answer TokenAnswer) {
	tokens.token = answer.AccessToken
	tokens.expiry = tokens.now().Add(time.Duration(answer.ExpiresIn)*time.Second - tokens.margin())
}

func (*ClientCredentialTokens) margin() time.Duration {
	return 30 * time.Second
}
