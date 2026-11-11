package keycloak

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Admin struct {
	settings AdminSettings
	client   *http.Client
}

func NewAdmin(settings AdminSettings, client *http.Client) Admin {
	return Admin{settings: settings, client: client}
}

func (admin Admin) CreateRealm(realm string) error {
	token, err := admin.token()
	if err != nil {
		return err
	}
	body, err := Template{}.Realm(realm, admin.settings.Secrets)
	if err != nil {
		return err
	}
	status, _, err := admin.exchange(http.MethodPost, admin.addresses().Realms(), token, body)
	if err != nil {
		return err
	}
	return admin.createOutcome(status, realm)
}

func (admin Admin) RealmExists(realm string) (bool, error) {
	err := admin.reachable()
	if err != nil {
		return false, err
	}
	request, err := http.NewRequest(http.MethodGet, admin.addresses().Discovery(realm), nil)
	if err != nil {
		return false, admin.failed(err)
	}
	status, _, err := admin.perform(request)
	if err != nil {
		return false, err
	}
	return admin.existsOutcome(status)
}

func (admin Admin) DeleteRealm(realm string) error {
	token, err := admin.token()
	if err != nil {
		return err
	}
	status, _, err := admin.exchange(http.MethodDelete, admin.addresses().Realm(realm), token, nil)
	if err != nil {
		return err
	}
	return admin.deleteOutcome(status, realm)
}

func (admin Admin) token() (string, error) {
	err := admin.configured()
	if err != nil {
		return "", err
	}
	request, err := http.NewRequest(http.MethodPost, admin.addresses().Token(), strings.NewReader(admin.tokenForm()))
	if err != nil {
		return "", admin.failed(err)
	}
	request.Header.Set(admin.contentTypeHeader(), admin.formType())
	status, content, err := admin.perform(request)
	if err != nil {
		return "", err
	}
	err = admin.expected(status, admin.tokenAction())
	if err != nil {
		return "", err
	}
	return admin.accessToken(content)
}

func (admin Admin) reachable() error {
	if admin.settings.BaseUrl == "" {
		return NotConfigured{}
	}
	return nil
}

func (admin Admin) configured() error {
	if admin.settings.BaseUrl == "" || admin.settings.ClientId == "" || admin.settings.ClientSecret == "" {
		return NotConfigured{}
	}
	return nil
}

func (admin Admin) tokenForm() string {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", admin.settings.ClientId)
	form.Set("client_secret", admin.settings.ClientSecret)
	return form.Encode()
}

func (admin Admin) accessToken(content []byte) (string, error) {
	var answer tokenAnswer
	err := json.Unmarshal(content, &answer)
	if err != nil {
		return "", fmt.Errorf("the Keycloak token answer is not valid JSON: %w", err)
	}
	return admin.nonEmptyToken(answer.AccessToken)
}

func (admin Admin) nonEmptyToken(token string) (string, error) {
	if token == "" {
		return "", AdminRefused{Status: http.StatusOK, Action: admin.tokenAction()}
	}
	return token, nil
}

func (admin Admin) exchange(method string, address string, token string, body []byte) (int, []byte, error) {
	request, err := http.NewRequest(method, address, bytes.NewReader(body))
	if err != nil {
		return 0, nil, admin.failed(err)
	}
	request.Header.Set(admin.authorizationHeader(), admin.bearerPrefix()+token)
	request.Header.Set(admin.contentTypeHeader(), admin.jsonType())
	return admin.perform(request)
}

func (admin Admin) perform(request *http.Request) (int, []byte, error) {
	response, err := admin.client.Do(request)
	if err != nil {
		return 0, nil, admin.failed(err)
	}
	content, readError := io.ReadAll(io.LimitReader(response.Body, admin.maxAnswerBytes()))
	closeError := response.Body.Close()
	return response.StatusCode, content, admin.failed(errors.Join(readError, closeError))
}

func (Admin) failed(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("the Keycloak admin request failed: %w", err)
}

func (Admin) expected(status int, action string) error {
	if status != http.StatusOK {
		return AdminRefused{Status: status, Action: action}
	}
	return nil
}

func (admin Admin) createOutcome(status int, realm string) error {
	switch status {
	case http.StatusCreated:
		return nil
	case http.StatusConflict:
		return RealmExists{Realm: realm}
	case http.StatusForbidden:
		return admin.forbiddenCreate(realm)
	default:
		return AdminRefused{Status: status, Action: admin.createAction()}
	}
}

func (admin Admin) forbiddenCreate(realm string) error {
	refused := AdminRefused{Status: http.StatusForbidden, Action: admin.createAction()}
	exists, err := admin.RealmExists(realm)
	if err != nil {
		return errors.Join(refused, err)
	}
	return admin.adoptable(exists, realm, refused)
}

func (Admin) adoptable(exists bool, realm string, refused AdminRefused) error {
	if exists {
		return RealmExists{Realm: realm}
	}
	return refused
}

func (admin Admin) existsOutcome(status int) (bool, error) {
	switch status {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, AdminRefused{Status: status, Action: admin.readAction()}
	}
}

func (admin Admin) deleteOutcome(status int, realm string) error {
	switch status {
	case http.StatusNoContent, http.StatusNotFound:
		return nil
	case http.StatusForbidden:
		return RealmNotManaged{Realm: realm}
	default:
		return AdminRefused{Status: status, Action: admin.deleteAction()}
	}
}

func (admin Admin) addresses() adminAddresses {
	return adminAddresses{base: admin.settings.BaseUrl}
}

func (Admin) maxAnswerBytes() int64 {
	return 1 << 20
}

func (Admin) authorizationHeader() string {
	return "Authorization"
}

func (Admin) bearerPrefix() string {
	return "Bearer "
}

func (Admin) contentTypeHeader() string {
	return "Content-Type"
}

func (Admin) formType() string {
	return "application/x-www-form-urlencoded"
}

func (Admin) jsonType() string {
	return "application/json"
}

func (Admin) tokenAction() string {
	return "issue an admin access token"
}

func (Admin) createAction() string {
	return "create the realm"
}

func (Admin) readAction() string {
	return "read the realm discovery document"
}

func (Admin) deleteAction() string {
	return "delete the realm"
}
