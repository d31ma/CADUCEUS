package oidc

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

type KeycloakAdmin struct {
	Client   *http.Client
	Endpoint string
	Token    string
}

func (admin KeycloakAdmin) ResetPassword(userId string, password string) error {
	return admin.send(http.MethodPut, admin.user(userId)+"/reset-password", PasswordReset{Type: "password", Value: password}, nil)
}

func (admin KeycloakAdmin) Sessions(userId string) ([]UserSession, error) {
	sessions := []UserSession{}
	return sessions, admin.send(http.MethodGet, admin.user(userId)+"/sessions", nil, &sessions)
}

func (admin KeycloakAdmin) EndSession(sessionId string) error {
	return admin.send(http.MethodDelete, "/sessions/"+url.PathEscape(sessionId), nil, nil)
}

func (admin KeycloakAdmin) Credentials(userId string) ([]UserCredential, error) {
	credentials := []UserCredential{}
	return credentials, admin.send(http.MethodGet, admin.user(userId)+"/credentials", nil, &credentials)
}

func (admin KeycloakAdmin) DeleteCredential(userId string, credentialId string) error {
	return admin.send(http.MethodDelete, admin.user(userId)+"/credentials/"+url.PathEscape(credentialId), nil, nil)
}

func (admin KeycloakAdmin) SetOtp(userId string, secret string) error {
	credential := OtpCredential{Secret: secret, Digits: Totp{}.values().Digits, Period: Totp{}.values().PeriodSeconds, Algorithm: "HmacSHA1"}
	return admin.send(http.MethodPut, "/mesail"+admin.user(userId)+"/otp", credential, nil)
}

func (KeycloakAdmin) user(userId string) string {
	return "/users/" + url.PathEscape(userId)
}

func (admin KeycloakAdmin) send(method string, path string, body any, into any) error {
	request, err := admin.request(method, path, body)
	if err != nil {
		return err
	}
	response, err := admin.Client.Do(request)
	if err != nil {
		return KeycloakUnavailable{}
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, HttpKeyFetcher{}.maximumBytes()))
	if err != nil {
		return KeycloakUnavailable{Status: response.StatusCode}
	}
	err = admin.answered(response.StatusCode, KeycloakClient{}.failure(raw))
	if err != nil {
		return err
	}
	return admin.decoded(raw, into)
}

func (admin KeycloakAdmin) request(method string, path string, body any) (*http.Request, error) {
	payload, err := admin.payload(body)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(method, admin.Endpoint+path, payload)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", BearerToken{}.prefix()+admin.Token)
	request.Header.Set("Content-Type", "application/json")
	return request, nil
}

func (KeycloakAdmin) payload(body any) (io.Reader, error) {
	if body == nil {
		return http.NoBody, nil
	}
	raw, err := json.Marshal(body)
	return bytes.NewReader(raw), err
}

func (KeycloakAdmin) answered(status int, failure KeycloakError) error {
	switch {
	case status < http.StatusMultipleChoices:
		return nil
	case status == http.StatusBadRequest:
		return AdminRefused{Message: failure.Description}
	case status == http.StatusNotFound:
		return SignInRefused{}
	}
	return KeycloakUnavailable{Status: status}
}

func (KeycloakAdmin) decoded(raw []byte, into any) error {
	if into == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, into)
}
