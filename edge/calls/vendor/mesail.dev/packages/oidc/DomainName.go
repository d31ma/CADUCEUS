package oidc

import "regexp"

type DomainName struct{}

func (name DomainName) Valid(domainId string) bool {
	return len(domainId) <= name.maximumLength() && name.pattern().MatchString(domainId)
}

func (DomainName) pattern() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)
}

func (DomainName) maximumLength() int {
	return 253
}
