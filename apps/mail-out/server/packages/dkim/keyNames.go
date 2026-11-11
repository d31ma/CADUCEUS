package dkim

import (
	"regexp"
	"strings"
)

type keyNames struct{}

func (keyNames) label() string {
	return `[a-z0-9_-]+(?:\.[a-z0-9_-]+)*`
}

func (names keyNames) secretPattern() *regexp.Regexp {
	return regexp.MustCompile(`^` + names.label() + `(?:/` + names.label() + `)*$`)
}

func (names keyNames) pairPattern() *regexp.Regexp {
	return regexp.MustCompile(`^` + names.label() + `/` + names.label() + `$`)
}

func (keyNames) prefix() string {
	return "dkim/"
}

func (names keyNames) secretName(domain string, selector string) string {
	return names.prefix() + strings.ToLower(domain) + "/" + strings.ToLower(selector)
}

func (names keyNames) safeSecret(name string) bool {
	return names.secretPattern().MatchString(name)
}

func (names keyNames) checkSecret(name string) error {
	if names.safeSecret(name) {
		return nil
	}
	return UnsafeSecretName{Name: name}
}

func (names keyNames) checkKey(key Key) error {
	pair := strings.ToLower(key.Domain) + "/" + strings.ToLower(key.Selector)
	if names.pairPattern().MatchString(pair) {
		return nil
	}
	return InvalidKeyName{Domain: key.Domain, Selector: key.Selector}
}
