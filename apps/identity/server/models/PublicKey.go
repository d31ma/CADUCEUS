type PublicKey struct {
	KeyType   string `json:"kty"`
	Curve     string `json:"crv"`
	X         string `json:"x"`
	Y         string `json:"y"`
	KeyId     string `json:"kid"`
	Algorithm string `json:"alg"`
	Use       string `json:"use"`
}
