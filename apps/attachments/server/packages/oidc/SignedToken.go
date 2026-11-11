package oidc

type SignedToken struct {
	Header       TokenHeader
	Claims       Claims
	SigningInput string
	Signature    []byte
}
