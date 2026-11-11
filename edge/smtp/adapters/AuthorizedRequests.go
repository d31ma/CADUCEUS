package adapters

import (
	"context"
	"io"
	"net/http"

	"mesail.dev/edge/smtp/reception"
)

type AuthorizedRequests struct {
	tokens reception.ServiceTokens
}

func NewAuthorizedRequests(tokens reception.ServiceTokens) AuthorizedRequests {
	return AuthorizedRequests{tokens: tokens}
}

func (requests AuthorizedRequests) New(ctx context.Context, method string, address string, body io.Reader) (*http.Request, error) {
	token, err := requests.tokens.Token(ctx)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, method, address, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", requests.JsonType())
	return request, nil
}

func (AuthorizedRequests) Finish(response *http.Response) {
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
}

func (AuthorizedRequests) JsonType() string {
	return "application/json"
}
