package oidc

type InstallWide struct{}

func (InstallWide) DomainId() string {
	return "*"
}

func (wide InstallWide) Matches(domainId string) bool {
	return domainId == wide.DomainId()
}
