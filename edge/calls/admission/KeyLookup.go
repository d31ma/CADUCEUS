package admission

import "mesail.dev/packages/oidc"

type KeyLookup interface {
	Key(keyId string) (oidc.JsonWebKey, error)
}
