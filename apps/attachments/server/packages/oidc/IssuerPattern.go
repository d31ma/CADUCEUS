package oidc

import (
	"regexp"
	"strings"
)

type IssuerPattern struct {
	Pattern string
}

func (pattern IssuerPattern) Realm(issuer string) (string, error) {
	prefix, suffix, found := strings.Cut(pattern.Pattern, pattern.placeholder())
	name, hasPrefix := strings.CutPrefix(issuer, prefix)
	name, hasSuffix := strings.CutSuffix(name, suffix)
	return pattern.allowed(name, found && hasPrefix && hasSuffix, issuer)
}

func (pattern IssuerPattern) Expand(realm string) (string, error) {
	_, err := pattern.allowed(realm, strings.Contains(pattern.Pattern, pattern.placeholder()), realm)
	return strings.ReplaceAll(pattern.Pattern, pattern.placeholder(), realm), err
}

func (pattern IssuerPattern) allowed(name string, matched bool, issuer string) (string, error) {
	if !matched || len(name) > pattern.maximumNameLength() || !pattern.realmName().MatchString(name) {
		return "", UnknownIssuer{Issuer: issuer}
	}
	return name, nil
}

func (IssuerPattern) realmName() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)
}

func (IssuerPattern) placeholder() string {
	return "{realm}"
}

func (IssuerPattern) maximumNameLength() int {
	return 253
}
