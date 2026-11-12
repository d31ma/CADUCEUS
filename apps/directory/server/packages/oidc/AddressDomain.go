package oidc

import "strings"

type AddressDomain struct{}

func (domain AddressDomain) Of(address string) (string, error) {
	_, name, _ := strings.Cut(strings.ToLower(strings.TrimSpace(address)), "@")
	return name, domain.problem(name)
}

func (AddressDomain) problem(name string) error {
	if !(DomainName{}).Valid(name) {
		return UnknownIssuer{Issuer: name}
	}
	return nil
}
