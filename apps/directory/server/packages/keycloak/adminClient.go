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

type adminClient struct {
	settings UserSettings
	client   *http.Client
}

func (admin adminClient) read(method string, path string) (adminAnswer, error) {
	return admin.exchange(method, path, nil)
}

func (admin adminClient) write(method string, path string, body any) (adminAnswer, error) {
	content, err := json.Marshal(body)
	if err != nil {
		return adminAnswer{}, admin.failed(err)
	}
	return admin.exchange(method, path, content)
}

func (adminClient) decode(content []byte, target any) error {
	err := json.Unmarshal(content, target)
	if err != nil {
		return fmt.Errorf("the Keycloak answer is not valid JSON: %w", err)
	}
	return nil
}

func (admin adminClient) exchange(method string, path string, content []byte) (adminAnswer, error) {
	token, err := admin.token()
	if err != nil {
		return adminAnswer{}, err
	}
	request, err := http.NewRequest(method, admin.addresses().Admin(path), bytes.NewReader(content))
	if err != nil {
		return adminAnswer{}, admin.failed(err)
	}
	request.Header.Set(admin.authorizationHeader(), admin.bearerPrefix()+token)
	request.Header.Set(admin.contentTypeHeader(), admin.jsonType())
	return admin.perform(request)
}

func (admin adminClient) token() (string, error) {
	err := admin.configured()
	if err != nil {
		return "", err
	}
	request, err := http.NewRequest(http.MethodPost, admin.addresses().Token(), strings.NewReader(admin.tokenForm()))
	if err != nil {
		return "", admin.failed(err)
	}
	request.Header.Set(admin.contentTypeHeader(), admin.formType())
	answer, err := admin.perform(request)
	if err != nil {
		return "", err
	}
	err = admin.tokenIssued(answer.Status)
	if err != nil {
		return "", err
	}
	var token tokenAnswer
	err = admin.decode(answer.Content, &token)
	if err != nil {
		return "", err
	}
	return admin.nonEmptyToken(token.AccessToken)
}

func (admin adminClient) configured() error {
	if admin.settings.BaseUrl == "" || admin.settings.ClientId == "" || admin.settings.ClientSecret == "" {
		return NotConfigured{}
	}
	return nil
}

func (admin adminClient) tokenForm() string {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", admin.settings.ClientId)
	form.Set("client_secret", admin.settings.ClientSecret)
	return form.Encode()
}

func (admin adminClient) tokenIssued(status int) error {
	if status != http.StatusOK {
		return AdminRefused{Status: status, Action: admin.tokenAction()}
	}
	return nil
}

func (admin adminClient) nonEmptyToken(token string) (string, error) {
	if token == "" {
		return "", AdminRefused{Status: http.StatusOK, Action: admin.tokenAction()}
	}
	return token, nil
}

func (admin adminClient) perform(request *http.Request) (adminAnswer, error) {
	response, err := admin.client.Do(request)
	if err != nil {
		return adminAnswer{}, admin.failed(err)
	}
	content, readError := io.ReadAll(io.LimitReader(response.Body, admin.maxAnswerBytes()))
	closeError := response.Body.Close()
	answer := adminAnswer{Status: response.StatusCode, Content: content, Location: response.Header.Get(admin.locationHeader())}
	return answer, errors.Join(admin.failed(errors.Join(readError, closeError)), admin.available(answer.Status))
}

func (adminClient) failed(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("the Keycloak admin request failed: %w", err)
}

func (adminClient) available(status int) error {
	if status >= http.StatusInternalServerError {
		return Unavailable{Status: status}
	}
	return nil
}

func (admin adminClient) addresses() adminAddresses {
	return adminAddresses{base: admin.settings.BaseUrl}
}

func (adminClient) maxAnswerBytes() int64 {
	return 1 << 20
}

func (adminClient) authorizationHeader() string {
	return "Authorization"
}

func (adminClient) bearerPrefix() string {
	return "Bearer "
}

func (adminClient) contentTypeHeader() string {
	return "Content-Type"
}

func (adminClient) locationHeader() string {
	return "Location"
}

func (adminClient) formType() string {
	return "application/x-www-form-urlencoded"
}

func (adminClient) jsonType() string {
	return "application/json"
}

func (adminClient) tokenAction() string {
	return "issue an admin access token"
}
