package oidc

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type KeycloakClient struct {
	Client       *http.Client
	Endpoint     string
	ClientId     string
	ClientSecret string
}

func (keycloak KeycloakClient) SignIn(username string, password string, oneTimeCode string) (TokenGrant, error) {
	form := keycloak.form("password")
	form.Set("username", username)
	form.Set("password", password)
	form.Set("totp", oneTimeCode)
	form.Set("scope", "openid")
	return keycloak.grant(form)
}

func (keycloak KeycloakClient) PasskeySignIn(credentialJson string, challenge string, origins string) (TokenGrant, error) {
	form := keycloak.form("password")
	form.Set("passkey_assertion", credentialJson)
	form.Set("passkey_challenge", challenge)
	form.Set("passkey_origins", origins)
	form.Set("scope", "openid")
	return keycloak.grant(form)
}

func (keycloak KeycloakClient) Refresh(refreshToken string) (TokenGrant, error) {
	form := keycloak.form(keycloak.refreshField())
	form.Set(keycloak.refreshField(), refreshToken)
	return keycloak.grant(form)
}

func (keycloak KeycloakClient) ClientCredentials() (TokenGrant, error) {
	return keycloak.grant(keycloak.form("client_credentials"))
}

func (keycloak KeycloakClient) SignOut(refreshToken string) error {
	form := keycloak.credentials()
	form.Set(keycloak.refreshField(), refreshToken)
	_, err := keycloak.post("/logout", form)
	return err
}

func (keycloak KeycloakClient) form(grantType string) url.Values {
	form := keycloak.credentials()
	form.Set("grant_type", grantType)
	return form
}

func (keycloak KeycloakClient) credentials() url.Values {
	return url.Values{"client_id": {keycloak.ClientId}, "client_secret": {keycloak.ClientSecret}}
}

func (keycloak KeycloakClient) grant(form url.Values) (TokenGrant, error) {
	body, err := keycloak.post("/token", form)
	if err != nil {
		return TokenGrant{}, err
	}
	grant := TokenGrant{}
	return grant, json.Unmarshal(body, &grant)
}

func (keycloak KeycloakClient) post(path string, form url.Values) ([]byte, error) {
	response, err := keycloak.Client.Post(keycloak.Endpoint+path, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, KeycloakUnavailable{}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, HttpKeyFetcher{}.maximumBytes()))
	if err != nil {
		return nil, KeycloakUnavailable{Status: response.StatusCode}
	}
	return body, keycloak.answered(response.StatusCode)
}

func (KeycloakClient) failure(body []byte) KeycloakError {
	failure := KeycloakError{}
	_ = json.Unmarshal(body, &failure)
	return failure
}

func (KeycloakClient) answered(status int) error {
	switch {
	case status < http.StatusMultipleChoices:
		return nil
	case status >= http.StatusInternalServerError:
		return KeycloakUnavailable{Status: status}
	}
	return SignInRefused{}
}

func (KeycloakClient) refreshField() string {
	return "refresh_token"
}
