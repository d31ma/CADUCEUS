package login

import (
	"cmp"

	"mesail.dev/packages/addresses"
	"mesail.dev/packages/oidc"
)

type ServiceTokenIssuer struct {
	Check  oidc.ServiceClientCheck
	Signer Signer
}

func (issuer ServiceTokenIssuer) Issue(clientId string, clientSecret string, domainId string) (oidc.IssuedToken, error) {
	err := issuer.accepted(clientId, clientSecret)
	if err != nil {
		return oidc.IssuedToken{}, err
	}
	domain, err := issuer.domain(domainId)
	if err != nil {
		return oidc.IssuedToken{}, err
	}
	return issuer.Signer.Sign(issuer.InstallWideToken(domain))
}

func (ServiceTokenIssuer) InstallWideToken(domainId string) oidc.MesailToken {
	return oidc.MesailToken{Subject: oidc.AuthorizedParties{}.ServiceSubject(), AuthorizedParty: oidc.AuthorizedParties{}.Service(), Domain: domainId, RealmRoles: []string{oidc.RealmRoles{}.Service()}}
}

func (issuer ServiceTokenIssuer) accepted(clientId string, clientSecret string) error {
	if !issuer.Check.Accepts(clientId, clientSecret) {
		return oidc.SignInRefused{}
	}
	return nil
}

func (issuer ServiceTokenIssuer) domain(domainId string) (string, error) {
	name, err := addresses.DomainName{}.Normalized(domainId)
	return issuer.allowedDomain(domainId, name, err)
}

func (ServiceTokenIssuer) allowedDomain(domainId string, name string, err error) (string, error) {
	if err != nil && !(oidc.InstallWide{}).Matches(domainId) {
		return "", InvalidDomain{}
	}
	return cmp.Or(name, domainId), nil
}
