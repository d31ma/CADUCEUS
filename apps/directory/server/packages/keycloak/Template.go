package keycloak

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed realm-template.json
var realmTemplate []byte

type Template struct{}

func (template Template) Realm(realm string, secrets RealmSecrets) ([]byte, error) {
	err := template.complete(secrets)
	if err != nil {
		return nil, err
	}
	pairs := []string{}
	for placeholder, value := range template.values(realm, secrets) {
		pairs = append(pairs, placeholder, template.escaped(value))
	}
	return []byte(strings.NewReplacer(pairs...).Replace(string(realmTemplate))), nil
}

func (template Template) values(realm string, secrets RealmSecrets) map[string]string {
	return map[string]string{
		template.realmPlaceholder():    realm,
		template.identityPlaceholder(): secrets.Identity,
		template.servicePlaceholder():  secrets.Service,
		template.accountPlaceholder():  secrets.Account,
	}
}

func (Template) complete(secrets RealmSecrets) error {
	if secrets.Identity == "" || secrets.Service == "" || secrets.Account == "" {
		return NotConfigured{}
	}
	return nil
}

func (template Template) escaped(value string) string {
	quoted, _ := json.Marshal(value)
	return strings.TrimSuffix(strings.TrimPrefix(string(quoted), template.quote()), template.quote())
}

func (Template) realmPlaceholder() string {
	return "${MESAIL_REALM}"
}

func (Template) identityPlaceholder() string {
	return "${MESAIL_IDENTITY_CLIENT_SECRET}"
}

func (Template) servicePlaceholder() string {
	return "${MESAIL_SERVICE_SECRET}"
}

func (Template) accountPlaceholder() string {
	return "${MESAIL_ACCOUNT_CLIENT_SECRET}"
}

func (Template) quote() string {
	return `"`
}
