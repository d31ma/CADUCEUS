package oidc

import "testing"

func TestPasskeyCredentialRemovesThePrfResultAndKeepsTheRest(t *testing.T) {
	cleaned, err := PasskeyCredential{}.WithoutPrf(`{"id":"abc","clientExtensionResults":{"credProps":{"rk":true},"prf":{"enabled":true,"results":{"first":"c2VjcmV0"}}},"response":{"signature":"c2ln"}}`, InvalidPasskey{})
	realmIs(t, err, nil)
	realmEqual(t, "cleaned", cleaned, `{"clientExtensionResults":{"credProps":{"rk":true}},"id":"abc","response":{"signature":"c2ln"}}`)
}

func TestPasskeyCredentialRemovesPrfInputsToo(t *testing.T) {
	cleaned, err := PasskeyCredential{}.WithoutPrf(`{"clientExtensionResults":{"prf":{"eval":{"first":"c2FsdA"}}}}`, InvalidPasskey{})
	realmIs(t, err, nil)
	realmEqual(t, "cleaned", cleaned, `{"clientExtensionResults":{}}`)
}

func TestPasskeyCredentialKeepsACredentialWithoutExtensionResults(t *testing.T) {
	cleaned, err := PasskeyCredential{}.WithoutPrf(`{"id":"abc","type":"public-key"}`, InvalidPasskey{})
	realmIs(t, err, nil)
	realmEqual(t, "cleaned", cleaned, `{"id":"abc","type":"public-key"}`)
}

func TestPasskeyCredentialKeepsOnlyTheLastOfTwoExtensionResults(t *testing.T) {
	cleaned, err := PasskeyCredential{}.WithoutPrf(`{"clientExtensionResults":{"credProps":{"rk":true}},"clientExtensionResults":{"prf":{"results":{"first":"c2VjcmV0"}}}}`, InvalidPasskey{})
	realmIs(t, err, nil)
	realmEqual(t, "cleaned", cleaned, `{"clientExtensionResults":{}}`)
}

func TestPasskeyCredentialRefusesTextThatIsNotJson(t *testing.T) {
	cleaned, err := PasskeyCredential{}.WithoutPrf("not json", SignInRefused{})
	realmIs(t, err, SignInRefused{})
	realmEqual(t, "cleaned", cleaned, "")
}

func TestPasskeyCredentialRefusesAnArray(t *testing.T) {
	_, err := PasskeyCredential{}.WithoutPrf(`[{"clientExtensionResults":{"prf":{}}}]`, InvalidPasskey{})
	realmIs(t, err, InvalidPasskey{})
}

func TestPasskeyCredentialRefusesExtensionResultsThatAreNotAnObject(t *testing.T) {
	_, err := PasskeyCredential{}.WithoutPrf(`{"clientExtensionResults":"{\"prf\":{}}"}`, InvalidPasskey{})
	realmIs(t, err, InvalidPasskey{})
}

func TestPasskeyCredentialRemovesAPrfPartThatOnlySaysEnabled(t *testing.T) {
	cleaned, err := PasskeyCredential{}.WithoutPrf(`{"clientExtensionResults":{"credProps":{"rk":true},"prf":{"enabled":true}},"id":"abc"}`, SignInRefused{})
	realmIs(t, err, nil)
	realmEqual(t, "cleaned", cleaned, `{"clientExtensionResults":{"credProps":{"rk":true}},"id":"abc"}`)
}
