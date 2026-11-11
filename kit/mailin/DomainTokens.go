package mailin

import "strings"

type DomainTokens struct {
	Authorize func(realm string) (string, error)
}

func (tokens DomainTokens) TokenFor(domainId string) (string, error) {
	authorization, err := tokens.Authorize(domainId)
	return strings.TrimPrefix(authorization, tokens.bearerPrefix()), err
}

func (DomainTokens) bearerPrefix() string {
	return "Bearer "
}
