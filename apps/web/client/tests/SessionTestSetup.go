import (
	"encoding/json"
	"fmt"
	"strings"
)

// @TestSetup
type SessionTestSetup struct{}

func (setup SessionTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	configuration := NewBehaviorConfiguration()
	configuration.Http["identity"] = setup
	configuration.Http["IdentityClient"] = setup
	configuration.Http["Identity"] = setup
	return configuration, nil
}

func (fake SessionTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	route := request.Path[strings.LastIndex(request.Path, "/"):]
	switch route {
	case "/config":
		return fake.config(), nil
	case "/me":
		return fake.me(fmt.Sprint(request.Headers["Authorization"]))
	case "/token":
		return fake.token(fake.tokenRequestOf(request).Code)
	case "/refresh":
		return fake.refresh(fake.refreshRequestOf(request).RefreshToken)
	}
	return nil, fake.status(404)
}

func (fake SessionTestSetup) config() ClientConfig {
	return ClientConfig{
		Issuer:      "https://sign-in.example.test/realms/{domainId}",
		ClientId:    "mesail-web",
		Scopes:      []string{"openid", "email"},
		ApiBase:     "/api/v1",
		RedirectUri: "https://mesail.example.test/",
	}
}

func (fake SessionTestSetup) me(authorization string) (BehaviorValue, error) {
	switch authorization {
	case "Bearer access-1", "Bearer access-2":
		return Me{MemberId: "m-1", DomainId: "example.com", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.com", Roles: []string{}}, nil
	case "Bearer access-disabled":
		return nil, fake.status(403)
	}
	return nil, fake.status(401)
}

func (fake SessionTestSetup) token(code string) (BehaviorValue, error) {
	switch code {
	case "good-code":
		return SessionTokenAnswer{AccessToken: "access-1", RefreshToken: "refresh-1", IdToken: "id-1", ExpiresIn: 60}, nil
	case "disabled-code":
		return SessionTokenAnswer{AccessToken: "access-disabled", RefreshToken: "refresh-x", IdToken: "id-x", ExpiresIn: 60}, nil
	}
	return nil, fake.status(400)
}

func (fake SessionTestSetup) refresh(refreshToken string) (BehaviorValue, error) {
	if refreshToken == "refresh-1" {
		return SessionTokenAnswer{AccessToken: "access-2", RefreshToken: "refresh-2", IdToken: "id-2", ExpiresIn: 60}, nil
	}
	return nil, fake.status(400)
}

func (fake SessionTestSetup) tokenRequestOf(request *BehaviorHttpRequest) SessionTokenRequest {
	text, _ := json.Marshal(request.Body)
	body := SessionTokenRequest{}
	_ = json.Unmarshal(text, &body)
	return body
}

func (fake SessionTestSetup) refreshRequestOf(request *BehaviorHttpRequest) SessionRefreshRequest {
	text, _ := json.Marshal(request.Body)
	body := SessionRefreshRequest{}
	_ = json.Unmarshal(text, &body)
	return body
}

func (fake SessionTestSetup) status(code int) error {
	return &BehaviorError{Code: "HTTP_STATUS", Message: "fake answer", Status: code}
}
