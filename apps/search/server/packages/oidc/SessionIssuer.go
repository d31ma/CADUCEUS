package oidc

import (
	"encoding/json"
	"errors"
	"net/http"
)

type SessionIssuer struct {
	Settings SessionSettings
	Keycloak string
	Admin    string
	Sealer   SealedCookie
	Client   *http.Client
	Problem  error
}

func (issuer SessionIssuer) refusal(username string, refused error) error {
	locked := errors.Is(refused, SignInRefused{}) && issuer.locked(username)
	return issuer.lockedOr(locked, refused)
}

func (issuer SessionIssuer) locked(username string) bool {
	admin, err := issuer.admin()
	if err != nil {
		return false
	}
	userId, err := admin.UserId(username)
	if err != nil {
		return false
	}
	locked, err := admin.Locked(userId)
	return err == nil && locked
}

func (SessionIssuer) lockedOr(locked bool, refused error) error {
	if locked {
		return AccountLocked{}
	}
	return refused
}

func (issuer SessionIssuer) admin() (KeycloakAdmin, error) {
	keycloak := issuer.keycloak()
	keycloak.ClientId = issuer.accountClientId()
	keycloak.ClientSecret = issuer.Settings.Account.Secret
	grant, err := keycloak.ClientCredentials()
	if err != nil {
		return KeycloakAdmin{}, err
	}
	return KeycloakAdmin{Client: issuer.Client, Endpoint: issuer.Admin, Token: grant.AccessToken}, nil
}

func (issuer SessionIssuer) opened(sessionKey string) (SessionContent, error) {
	if issuer.Problem != nil {
		return SessionContent{}, issuer.Problem
	}
	plain, err := issuer.Sealer.Open(sessionKey, issuer.sealLabel())
	if err != nil {
		return SessionContent{}, err
	}
	content := SessionContent{}
	return content, json.Unmarshal([]byte(plain), &content)
}

func (issuer SessionIssuer) keycloak() KeycloakClient {
	return KeycloakClient{Client: issuer.Client, Endpoint: issuer.Keycloak, ClientId: issuer.clientId(), ClientSecret: issuer.Settings.Client.Secret}
}

func (SessionIssuer) accountClientId() string {
	return "mesail-account"
}

func (SessionIssuer) clientId() string {
	return "mesail-identity"
}

func (SessionIssuer) sealLabel() string {
	return "mesail-session"
}
