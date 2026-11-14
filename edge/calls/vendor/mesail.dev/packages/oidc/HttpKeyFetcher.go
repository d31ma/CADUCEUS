package oidc

import (
	"encoding/json"
	"io"
	"net/http"
)

type HttpKeyFetcher struct {
	Client *http.Client
}

func (fetcher HttpKeyFetcher) Fetch(jwksUrl string) (KeySet, error) {
	response, err := fetcher.Client.Get(jwksUrl)
	if err != nil {
		return KeySet{}, KeycloakUnavailable{}
	}
	defer response.Body.Close()
	err = fetcher.succeeded(response)
	if err != nil {
		return KeySet{}, err
	}
	set := KeySet{}
	err = json.NewDecoder(io.LimitReader(response.Body, fetcher.maximumBytes())).Decode(&set)
	return set, err
}

func (HttpKeyFetcher) succeeded(response *http.Response) error {
	if response.StatusCode != http.StatusOK {
		return KeycloakUnavailable{Status: response.StatusCode}
	}
	return nil
}

func (HttpKeyFetcher) maximumBytes() int64 {
	return 1 << 20
}
