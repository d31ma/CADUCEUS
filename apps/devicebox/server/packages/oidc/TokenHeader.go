package oidc

type TokenHeader struct {
	Algorithm string `json:"alg"`
	KeyId     string `json:"kid"`
	Type      string `json:"typ"`
}

func (header TokenHeader) Supported() error {
	return header.checked(SignatureCheck{}.Supports(header.Algorithm))
}

func (TokenHeader) checked(supported bool) error {
	if !supported {
		return RefusedToken{Reason: "the signing algorithm is not RS256 or ES256"}
	}
	return nil
}
