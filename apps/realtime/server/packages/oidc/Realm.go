package oidc

import "strings"

type Realm struct{}

func (realm Realm) Name(issuer string) (string, error) {
	_, name, found := strings.Cut(strings.TrimSuffix(issuer, "/"), "/realms/")
	return realm.checked(name, found, issuer)
}

func (Realm) checked(name string, found bool, issuer string) (string, error) {
	if !found || name == "" || strings.Contains(name, "/") {
		return "", UnknownIssuer{Issuer: issuer}
	}
	return name, nil
}
