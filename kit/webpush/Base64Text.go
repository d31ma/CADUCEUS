package webpush

import (
	"encoding/base64"
	"strings"
)

type Base64Text struct{}

func (Base64Text) Encode(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (text Base64Text) Decode(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(value, text.padding()))
}

func (Base64Text) padding() string {
	return "="
}
