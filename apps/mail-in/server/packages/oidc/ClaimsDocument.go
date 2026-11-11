package oidc

type ClaimsDocument struct {
	Issuer          string       `json:"iss"`
	Subject         string       `json:"sub"`
	Audience        AudienceList `json:"aud"`
	AuthorizedParty string       `json:"azp"`
	ExpiresAt       int64        `json:"exp"`
	NotBefore       int64        `json:"nbf"`
	TokenType       string       `json:"typ"`
	SessionId       string       `json:"sid"`
	Name            string       `json:"name"`
	Email           string       `json:"email"`
	RealmAccess     RealmAccess  `json:"realm_access"`
}

func (document ClaimsDocument) Claims() Claims {
	return Claims{
		Issuer:          document.Issuer,
		Subject:         document.Subject,
		Audience:        document.Audience,
		AuthorizedParty: document.AuthorizedParty,
		ExpiresAt:       document.ExpiresAt,
		NotBefore:       document.NotBefore,
		TokenType:       document.TokenType,
		SessionId:       document.SessionId,
		Name:            document.Name,
		Email:           document.Email,
		RealmRoles:      document.RealmAccess.Roles,
	}
}
