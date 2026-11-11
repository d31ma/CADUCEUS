package login

import "mesail.dev/packages/oidc"

type Signer interface {
	Sign(token oidc.MesailToken) (oidc.IssuedToken, error)
	KeySet() (oidc.KeySet, error)
}
