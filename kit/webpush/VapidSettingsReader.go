package webpush

import (
	"cmp"
	"crypto/ecdh"
	"encoding/json"
	"strings"
)

type VapidSettingsReader struct{}

func (reader VapidSettingsReader) FromApplicationJSON(document string) (VapidKeys, error) {
	keys, err := reader.parse(document)
	if err != nil {
		return VapidKeys{}, err
	}
	err = reader.check(keys)
	if err != nil {
		return VapidKeys{}, err
	}
	return keys, nil
}

func (VapidSettingsReader) parse(document string) (VapidKeys, error) {
	snapshot := SettingsDocument{}
	err := json.Unmarshal([]byte(document), &snapshot)
	if err != nil {
		return VapidKeys{}, NotConfigured{}
	}
	return snapshot.Keys(), nil
}

func (reader VapidSettingsReader) check(keys VapidKeys) error {
	present := reader.required(reader.present(keys))
	subject := reader.required(reader.subjectAllowed(keys.Subject))
	pair := reader.required(reader.pairMatches(keys))
	return cmp.Or(present, subject, pair)
}

func (VapidSettingsReader) required(valid bool) error {
	if !valid {
		return NotConfigured{}
	}
	return nil
}

func (VapidSettingsReader) present(keys VapidKeys) bool {
	return keys.PublicKey != "" && keys.PrivateKey != "" && keys.Subject != ""
}

func (reader VapidSettingsReader) subjectAllowed(subject string) bool {
	return strings.HasPrefix(subject, reader.mailScheme()) || strings.HasPrefix(subject, reader.webScheme())
}

func (reader VapidSettingsReader) pairMatches(keys VapidKeys) bool {
	derived, err := reader.derivedPublicKey(keys.PrivateKey)
	return err == nil && derived == keys.PublicKey
}

func (VapidSettingsReader) derivedPublicKey(privateKey string) (string, error) {
	scalar, err := Base64Text{}.Decode(privateKey)
	if err != nil {
		return "", err
	}
	key, err := ecdh.P256().NewPrivateKey(scalar)
	if err != nil {
		return "", err
	}
	return Base64Text{}.Encode(key.PublicKey().Bytes()), nil
}

func (VapidSettingsReader) mailScheme() string {
	return "mailto:"
}

func (VapidSettingsReader) webScheme() string {
	return "https:"
}
