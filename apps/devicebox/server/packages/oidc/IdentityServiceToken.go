package oidc

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

type IdentityServiceToken struct {
	Client   *http.Client
	Endpoint string
	Secret   string
	DomainId string
}

func (token IdentityServiceToken) CacheKey() string {
	return token.Endpoint + "#" + token.DomainId
}

func (token IdentityServiceToken) Fetch() (TokenGrant, error) {
	raw, err := token.post(token.body())
	if err != nil {
		return TokenGrant{}, err
	}
	answer := ServiceTokenAnswer{}
	err = json.Unmarshal(raw, &answer)
	return TokenGrant{AccessToken: answer.AccessToken, ExpiresIn: answer.ExpiresIn}, err
}

func (token IdentityServiceToken) body() []byte {
	body, _ := json.Marshal(ServiceTokenRequest{ClientId: AuthorizedParties{}.Service(), ClientSecret: token.Secret, DomainId: token.DomainId})
	return body
}

func (token IdentityServiceToken) post(body []byte) ([]byte, error) {
	response, err := token.Client.Post(token.Endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, KeycloakUnavailable{}
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, HttpKeyFetcher{}.maximumBytes()))
	if err != nil {
		return nil, KeycloakUnavailable{Status: response.StatusCode}
	}
	return raw, KeycloakClient{}.answered(response.StatusCode)
}
