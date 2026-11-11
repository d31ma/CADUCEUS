package adapters

import "context"

type StaticTokens struct {
	token string
}

func NewStaticTokens(token string) StaticTokens {
	return StaticTokens{token: token}
}

func (tokens StaticTokens) Token(_ context.Context) (string, error) {
	return tokens.token, nil
}
