package oidc

import (
	"net/http"
	"net/url"
	"slices"
)

type KeycloakPasskeys struct {
	Admin KeycloakAdmin
}

func (passkeys KeycloakPasskeys) List(userId string) ([]PasskeyRecord, error) {
	records := []PasskeyRecord{}
	return records, passkeys.send(http.MethodGet, passkeys.collection(userId), nil, &records)
}

func (passkeys KeycloakPasskeys) Add(userId string, request PasskeyAddRequest) (PasskeyRecord, error) {
	record := PasskeyRecord{}
	return record, passkeys.send(http.MethodPost, passkeys.collection(userId), request, &record)
}

func (passkeys KeycloakPasskeys) Rename(userId string, passkeyId string, label string) (PasskeyRecord, error) {
	record := PasskeyRecord{}
	return record, passkeys.send(http.MethodPut, passkeys.item(userId, passkeyId), PasskeyLabelRequest{Label: label}, &record)
}

func (passkeys KeycloakPasskeys) Remove(userId string, passkeyId string) error {
	return passkeys.send(http.MethodDelete, passkeys.item(userId, passkeyId), nil, nil)
}

func (passkeys KeycloakPasskeys) send(method string, path string, body any, into any) error {
	status, raw, err := passkeys.Admin.exchange(method, path, body)
	if err != nil {
		return err
	}
	err = passkeys.answered(status, KeycloakClient{}.failure(raw))
	if err != nil {
		return err
	}
	return passkeys.Admin.decoded(raw, into)
}

func (passkeys KeycloakPasskeys) answered(status int, failure KeycloakError) error {
	switch {
	case status < http.StatusMultipleChoices:
		return nil
	case status == http.StatusNotFound:
		return PasskeyNotFound{}
	case failure.Code == (LabelExists{}).PasskeyCode():
		return LabelExists{}
	case failure.Code == (TooManyPasskeys{}).PasskeyCode():
		return TooManyPasskeys{}
	case passkeys.refusal(status):
		return PasskeyRefused{Status: status, Code: failure.Code}
	}
	return KeycloakUnavailable{Status: status}
}

func (KeycloakPasskeys) refusal(status int) bool {
	return slices.Contains([]int{http.StatusBadRequest, http.StatusConflict}, status)
}

func (passkeys KeycloakPasskeys) collection(userId string) string {
	return "/mesail/users/" + url.PathEscape(userId) + "/passkeys"
}

func (passkeys KeycloakPasskeys) item(userId string, passkeyId string) string {
	return passkeys.collection(userId) + "/" + url.PathEscape(passkeyId)
}
