package oidc

import (
	"encoding/json"
	"errors"
	"net/http"
)

type SessionIssuer struct {
	Gate     Gate
	Settings SessionSettings
	Keycloak string
	Admin    string
	Sealer   SealedCookie
	Client   *http.Client
	Problem  error
}

func (issuer SessionIssuer) SignIn(address string, password string, oneTimeCode string) (SessionGrant, error) {
	grant, err := issuer.signIn(address, password, oneTimeCode)
	return grant, SessionErrors{}.Normalized(err)
}

func (issuer SessionIssuer) Refresh(sessionKey string) (SessionGrant, error) {
	grant, err := issuer.refresh(sessionKey)
	return grant, SessionErrors{}.Normalized(err)
}

func (issuer SessionIssuer) SignOut(sessionKey string) error {
	return SessionErrors{}.Normalized(issuer.signOut(sessionKey))
}

func (issuer SessionIssuer) signIn(address string, password string, oneTimeCode string) (SessionGrant, error) {
	if issuer.Problem != nil {
		return SessionGrant{}, issuer.Problem
	}
	domain, err := AddressDomain{}.Of(address)
	if err != nil {
		return SessionGrant{}, err
	}
	keycloak, err := issuer.keycloak(domain)
	if err != nil {
		return SessionGrant{}, err
	}
	tokens, err := keycloak.SignIn(address, password, oneTimeCode)
	if err != nil {
		return SessionGrant{}, issuer.refusal(domain, address, err)
	}
	return issuer.granted(domain, tokens)
}

func (issuer SessionIssuer) refusal(domain string, address string, refused error) error {
	locked := errors.Is(refused, SignInRefused{}) && issuer.locked(domain, address)
	return issuer.lockedOr(locked, refused)
}

func (issuer SessionIssuer) locked(domain string, address string) bool {
	admin, err := issuer.admin(domain)
	if err != nil {
		return false
	}
	userId, err := admin.UserId(address)
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

func (issuer SessionIssuer) admin(domain string) (KeycloakAdmin, error) {
	keycloak, err := issuer.keycloak(domain)
	if err != nil {
		return KeycloakAdmin{}, err
	}
	keycloak.ClientId = issuer.accountClientId()
	keycloak.ClientSecret = issuer.Settings.Account.Secret
	grant, err := keycloak.ClientCredentials()
	if err != nil {
		return KeycloakAdmin{}, err
	}
	endpoint, err := IssuerPattern{Pattern: issuer.Admin}.Resolved(domain)
	return KeycloakAdmin{Client: issuer.Client, Endpoint: endpoint, Token: grant.AccessToken}, err
}

func (issuer SessionIssuer) refresh(sessionKey string) (SessionGrant, error) {
	content, err := issuer.opened(sessionKey)
	if err != nil {
		return SessionGrant{}, err
	}
	keycloak, err := issuer.keycloak(content.Domain)
	if err != nil {
		return SessionGrant{}, err
	}
	tokens, err := keycloak.Refresh(content.RefreshToken)
	if err != nil {
		return SessionGrant{}, err
	}
	return issuer.granted(content.Domain, tokens)
}

func (issuer SessionIssuer) signOut(sessionKey string) error {
	content, err := issuer.opened(sessionKey)
	if err != nil {
		return err
	}
	keycloak, err := issuer.keycloak(content.Domain)
	if err != nil {
		return err
	}
	return keycloak.SignOut(content.RefreshToken)
}

func (issuer SessionIssuer) granted(domain string, tokens TokenGrant) (SessionGrant, error) {
	caller, err := issuer.Gate.Identify(BearerToken{}.prefix() + tokens.AccessToken)
	if err != nil {
		return SessionGrant{}, err
	}
	content, err := json.Marshal(SessionContent{Domain: domain, RefreshToken: tokens.RefreshToken})
	if err != nil {
		return SessionGrant{}, err
	}
	sessionKey, err := issuer.Sealer.Seal(string(content), issuer.sealLabel())
	return SessionGrant{Caller: caller, AccessToken: tokens.AccessToken, ExpiresIn: tokens.ExpiresIn, SessionKey: sessionKey}, err
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

func (issuer SessionIssuer) keycloak(domain string) (KeycloakClient, error) {
	endpoint, err := IssuerPattern{Pattern: issuer.Keycloak}.Resolved(domain)
	return KeycloakClient{Client: issuer.Client, Endpoint: endpoint, ClientId: issuer.clientId(), ClientSecret: issuer.Settings.Client.Secret}, err
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
