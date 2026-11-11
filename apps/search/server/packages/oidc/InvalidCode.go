package oidc

type InvalidCode struct{}

func (InvalidCode) Error() string {
	return "the 2-step code is not right"
}
