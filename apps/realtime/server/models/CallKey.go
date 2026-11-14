type CallKey struct {
	KeyType   string `json:"kty"`
	KeyId     string `json:"kid"`
	Algorithm string `json:"alg"`
	Use       string `json:"use"`
	Curve     string `json:"crv"`
	X         string `json:"x"`
	Y         string `json:"y"`
}
