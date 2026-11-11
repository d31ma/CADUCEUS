package login

import (
	"testing"

	"mesail.dev/packages/oidc"
)

func TestSignerKeysFindsTheSignerKey(t *testing.T) {
	signer := testSigner(t)
	keyId, err := signer.Current.KeyId()
	failOn(t, err)
	key, err := SignerKeys{Signer: signer}.Key("https://identity.test/jwks", keyId)
	failOn(t, err)
	same(t, "key id", key.KeyId, keyId)
}

func TestSignerKeysRefusesAnUnknownKey(t *testing.T) {
	_, err := SignerKeys{Signer: testSigner(t)}.Key("https://identity.test/jwks", "unknown")
	same(t, "refused", err != nil, true)
}

func TestSignerKeysPassesASignerFailureThrough(t *testing.T) {
	_, err := SignerKeys{Signer: brokenSigner()}.Key("https://identity.test/jwks", "any")
	isError(t, err, oidc.NotConfigured{})
}
