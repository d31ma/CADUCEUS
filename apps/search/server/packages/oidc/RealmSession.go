package oidc

import (
	"encoding/json"
	"slices"
	"time"
)

type RealmSession struct {
	Sessions SessionIssuer
	Verifier TokenVerifier
	Now      func() time.Time
}

func (session RealmSession) SignIn(username string, password string, oneTimeCode string) (KeycloakSession, error) {
	opened, err := session.signIn(username, password, oneTimeCode)
	return opened, SessionErrors{}.Normalized(err)
}

func (session RealmSession) Refresh(refreshToken string) (KeycloakSession, error) {
	opened, err := session.refresh(refreshToken)
	return opened, SessionErrors{}.Normalized(err)
}

func (session RealmSession) SignOut(refreshToken string) error {
	return SessionErrors{}.Normalized(session.signOut(refreshToken))
}

func (session RealmSession) Seal(content SessionContent) (string, error) {
	sealed, err := session.seal(content)
	return sealed, SessionErrors{}.Normalized(err)
}

func (session RealmSession) Open(sessionKey string) (SessionContent, error) {
	content, err := session.Sessions.opened(sessionKey)
	return content, SessionErrors{}.Normalized(err)
}

func (session RealmSession) signIn(username string, password string, oneTimeCode string) (KeycloakSession, error) {
	keycloak, err := session.keycloak()
	if err != nil {
		return KeycloakSession{}, err
	}
	tokens, err := keycloak.SignIn(username, password, oneTimeCode)
	if err != nil {
		return KeycloakSession{}, session.Sessions.refusal(session.realm(), username, err)
	}
	return session.opened(tokens)
}

func (session RealmSession) refresh(refreshToken string) (KeycloakSession, error) {
	keycloak, err := session.keycloak()
	if err != nil {
		return KeycloakSession{}, err
	}
	tokens, err := keycloak.Refresh(refreshToken)
	if err != nil {
		return KeycloakSession{}, err
	}
	return session.opened(tokens)
}

func (session RealmSession) signOut(refreshToken string) error {
	keycloak, err := session.keycloak()
	if err != nil {
		return err
	}
	return keycloak.SignOut(refreshToken)
}

func (session RealmSession) seal(content SessionContent) (string, error) {
	err := session.Sessions.Problem
	if err != nil {
		return "", err
	}
	plain, _ := json.Marshal(content)
	return session.Sessions.Sealer.Seal(string(plain), session.Sessions.sealLabel())
}

func (session RealmSession) opened(tokens TokenGrant) (KeycloakSession, error) {
	claims, err := session.Verifier.Verify(tokens.AccessToken, session.Now())
	if err != nil {
		return KeycloakSession{}, err
	}
	operator := slices.Contains(claims.RealmRoles, RealmRoles{}.Operator())
	return KeycloakSession{PersonId: claims.Subject, SessionId: claims.SessionId, Username: claims.Username, DisplayName: claims.Name, Operator: operator, MustChangePassword: claims.MustChangePassword, RefreshToken: tokens.RefreshToken}, nil
}

func (session RealmSession) keycloak() (KeycloakClient, error) {
	err := session.Sessions.Problem
	if err != nil {
		return KeycloakClient{}, err
	}
	return session.Sessions.keycloak(session.realm())
}

func (session RealmSession) admin() (KeycloakAdmin, error) {
	err := session.Sessions.Problem
	if err != nil {
		return KeycloakAdmin{}, err
	}
	return session.Sessions.admin(session.realm())
}

func (RealmSession) realm() string {
	return ""
}
