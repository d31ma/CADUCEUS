package oidc

type KeyThumbprintInput struct {
	Curve   string `json:"crv"`
	KeyType string `json:"kty"`
	X       string `json:"x"`
	Y       string `json:"y"`
}
