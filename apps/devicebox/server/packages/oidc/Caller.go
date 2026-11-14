package oidc

type Caller struct {
	DomainId    string
	MemberId    string
	Username    string
	DisplayName string
	Email       string
	Roles       []string
	SessionId   string
}
