package oidc

type Caller struct {
	DomainId    string
	MemberId    string
	DisplayName string
	Email       string
	Roles       []string
	SessionId   string
}
