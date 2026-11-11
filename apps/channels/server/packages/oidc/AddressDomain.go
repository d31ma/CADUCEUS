package oidc

import "strings"

type AddressDomain struct{}

func (AddressDomain) Of(address string) (string, error) {
	_, domain, _ := strings.Cut(strings.ToLower(strings.TrimSpace(address)), "@")
	_, err := IssuerPattern{Pattern: IssuerPattern{}.placeholder()}.Expand(domain)
	return domain, err
}
