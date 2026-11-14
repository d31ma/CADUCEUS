package oidc

import "encoding/json"

type PasskeyCredential struct{}

func (credential PasskeyCredential) WithoutPrf(credentialJson string, refused error) (string, error) {
	cleaned, err := credential.withoutPrf(credentialJson)
	return cleaned, credential.refusal(err, refused)
}

func (credential PasskeyCredential) withoutPrf(credentialJson string) (string, error) {
	fields, err := credential.object([]byte(credentialJson))
	if err != nil {
		return "", err
	}
	err = credential.removePrf(fields)
	if err != nil {
		return "", err
	}
	cleaned, err := json.Marshal(fields)
	return string(cleaned), err
}

func (credential PasskeyCredential) removePrf(fields map[string]json.RawMessage) error {
	raw, found := fields[credential.resultsField()]
	if !found {
		return nil
	}
	results, err := credential.object(raw)
	if err != nil {
		return err
	}
	delete(results, credential.prfField())
	fields[credential.resultsField()], err = json.Marshal(results)
	return err
}

func (PasskeyCredential) object(raw []byte) (map[string]json.RawMessage, error) {
	fields := map[string]json.RawMessage{}
	err := json.Unmarshal(raw, &fields)
	return fields, err
}

func (PasskeyCredential) refusal(err error, refused error) error {
	if err != nil {
		return refused
	}
	return nil
}

func (PasskeyCredential) resultsField() string {
	return "clientExtensionResults"
}

func (PasskeyCredential) prfField() string {
	return "prf"
}
