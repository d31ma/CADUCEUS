package oidc

type JsonWebKey struct {
	KeyType   string `json:"kty"`
	KeyId     string `json:"kid"`
	Algorithm string `json:"alg,omitempty"`
	Use       string `json:"use,omitempty"`
	Modulus   string `json:"n,omitempty"`
	Exponent  string `json:"e,omitempty"`
	Curve     string `json:"crv,omitempty"`
	X         string `json:"x,omitempty"`
	Y         string `json:"y,omitempty"`
}
