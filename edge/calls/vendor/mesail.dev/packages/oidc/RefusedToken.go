package oidc

type RefusedToken struct {
	Reason string
}

func (refused RefusedToken) Error() string {
	return "the token was refused: " + refused.Reason
}
