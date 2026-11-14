package oidc

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

type TokenParser struct{}

func (parser TokenParser) Parse(rawToken string) (SignedToken, error) {
	parts, err := parser.split(rawToken)
	if err != nil {
		return SignedToken{}, err
	}
	header := TokenHeader{}
	err = parser.segment(parts[0], &header)
	if err != nil {
		return SignedToken{}, err
	}
	err = header.Supported()
	if err != nil {
		return SignedToken{}, err
	}
	document := ClaimsDocument{}
	err = parser.segment(parts[1], &document)
	if err != nil {
		return SignedToken{}, err
	}
	signature, err := parser.decoded(parts[2])
	if err != nil {
		return SignedToken{}, err
	}
	return SignedToken{Header: header, Claims: document.Claims(), SigningInput: parts[0] + "." + parts[1], Signature: signature}, nil
}

func (parser TokenParser) split(rawToken string) ([]string, error) {
	parts := strings.Split(rawToken, ".")
	if len(rawToken) > parser.maximumLength() || len(parts) != parser.partCount() {
		return nil, RefusedToken{Reason: "the token is not a signed JWT"}
	}
	return parts, nil
}

func (parser TokenParser) segment(part string, into any) error {
	raw, err := parser.decoded(part)
	if err != nil {
		return err
	}
	return parser.unmarshalled(json.Unmarshal(raw, into))
}

func (TokenParser) decoded(part string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return nil, RefusedToken{Reason: "a token part is not base64url"}
	}
	return raw, nil
}

func (TokenParser) unmarshalled(err error) error {
	if err != nil {
		return RefusedToken{Reason: "a token part is not valid JSON"}
	}
	return nil
}

func (TokenParser) maximumLength() int {
	return 16384
}

func (TokenParser) partCount() int {
	return 3
}
