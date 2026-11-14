package oidc

import "strings"

type BearerToken struct{}

func (bearer BearerToken) FromHeader(header string) (string, error) {
	token, found := strings.CutPrefix(strings.TrimSpace(header), bearer.prefix())
	return bearer.nonEmpty(strings.TrimSpace(token), found)
}

func (BearerToken) prefix() string {
	return "Bearer "
}

func (BearerToken) nonEmpty(token string, found bool) (string, error) {
	if !found || token == "" {
		return "", MissingBearerToken{}
	}
	return token, nil
}
