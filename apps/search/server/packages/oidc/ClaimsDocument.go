package oidc

type ClaimsDocument struct {
	Issuer             string       `json:"iss"`
	Subject            string       `json:"sub"`
	Audience           AudienceList `json:"aud"`
	AuthorizedParty    string       `json:"azp"`
	ExpiresAt          int64        `json:"exp"`
	NotBefore          int64        `json:"nbf"`
	IssuedAt           int64        `json:"iat,omitempty"`
	TokenId            string       `json:"jti,omitempty"`
	TokenType          string       `json:"typ,omitempty"`
	SessionId          string       `json:"sid,omitempty"`
	Name               string       `json:"name,omitempty"`
	Username           string       `json:"preferred_username,omitempty"`
	Email              string       `json:"email,omitempty"`
	Domain             string       `json:"domain,omitempty"`
	RealmAccess        RealmAccess  `json:"realm_access"`
	MustChangePassword bool         `json:"mesail_must_change_password,omitempty"`
}

func (document ClaimsDocument) Claims() Claims {
	return Claims{
		Issuer:             document.Issuer,
		Subject:            document.Subject,
		Audience:           document.Audience,
		AuthorizedParty:    document.AuthorizedParty,
		ExpiresAt:          document.ExpiresAt,
		NotBefore:          document.NotBefore,
		TokenType:          document.TokenType,
		SessionId:          document.SessionId,
		Name:               document.Name,
		Email:              document.Email,
		RealmRoles:         document.RealmAccess.Roles,
		Domain:             document.Domain,
		Username:           document.Username,
		MustChangePassword: document.MustChangePassword,
	}
}
