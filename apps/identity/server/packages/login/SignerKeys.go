package login

import "mesail.dev/packages/oidc"

type SignerKeys struct {
	Signer Signer
}

func (keys SignerKeys) Key(jwksUrl string, keyId string) (oidc.JsonWebKey, error) {
	set, err := keys.Signer.KeySet()
	if err != nil {
		return oidc.JsonWebKey{}, err
	}
	return set.Find(keyId)
}
