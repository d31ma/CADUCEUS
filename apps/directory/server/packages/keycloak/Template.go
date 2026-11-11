package keycloak

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed realm-template.json
var realmTemplate []byte

type Template struct{}

func (template Template) Realm(realm string, publicUrl string) ([]byte, error) {
	escapedRealm, err := template.escaped(realm)
	if err != nil {
		return nil, err
	}
	escapedUrl, err := template.escaped(publicUrl)
	if err != nil {
		return nil, err
	}
	replacer := strings.NewReplacer(template.realmPlaceholder(), escapedRealm, template.publicUrlPlaceholder(), escapedUrl)
	return []byte(replacer.Replace(string(realmTemplate))), nil
}

func (template Template) escaped(value string) (string, error) {
	quoted, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	unquoted := strings.TrimSuffix(strings.TrimPrefix(string(quoted), template.quote()), template.quote())
	return unquoted, nil
}

func (Template) realmPlaceholder() string {
	return "${REALM}"
}

func (Template) publicUrlPlaceholder() string {
	return "${MESAIL_PUBLIC_URL}"
}

func (Template) quote() string {
	return `"`
}
