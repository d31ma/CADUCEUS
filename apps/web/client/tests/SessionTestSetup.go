import (
	"encoding/json"
	"errors"
)

// @TestSetup
type SessionTestSetup struct{}

func (setup SessionTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	configuration := NewBehaviorConfiguration()
	configuration.Http["identity"] = setup
	configuration.Http["IdentityClient"] = setup
	return configuration, nil
}

func (setup SessionTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	switch request.Method + " " + request.Path {
	case "POST /api/v1/identity/session":
		return setup.signIn(setup.signInRequestOf(request))
	case "POST /api/v1/identity/session/refresh":
		return setup.refresh(setup.refreshRequestOf(request).SessionKey)
	case "DELETE /api/v1/identity/session":
		return nil, nil
	}
	return nil, errors.New("no fake answer for " + request.Method + " " + request.Path)
}

func (setup SessionTestSetup) signIn(request SessionSignInRequest) (BehaviorValue, error) {
	switch request.Address + " " + request.Password + " " + request.OneTimeCode {
	case "ana@example.com right ", "ana@example.com right 123456":
		return setup.session("example.com", "access-1", "key-1"), nil
	case "ana@example.org right ":
		return setup.session("example.org", "access-org", "key-org"), nil
	}
	return nil, errors.New("unauthenticated")
}

func (setup SessionTestSetup) refresh(sessionKey string) (BehaviorValue, error) {
	switch sessionKey {
	case "key-1":
		return setup.session("example.com", "access-2", "key-2"), nil
	case "key-org":
		return setup.session("example.org", "access-org-2", "key-org-2"), nil
	}
	return nil, errors.New("unauthenticated")
}

func (setup SessionTestSetup) session(domainId string, accessToken string, sessionKey string) Session {
	me := Me{MemberId: "m-1", DomainId: domainId, DisplayName: "Ana Ruiz", PrimaryAddress: "ana@" + domainId, Roles: []string{}}
	return Session{Me: me, AccessToken: accessToken, ExpiresIn: 60, SessionKey: sessionKey}
}

func (setup SessionTestSetup) signInRequestOf(request *BehaviorHttpRequest) SessionSignInRequest {
	text, _ := json.Marshal(request.Body)
	body := SessionSignInRequest{}
	_ = json.Unmarshal(text, &body)
	return body
}

func (setup SessionTestSetup) refreshRequestOf(request *BehaviorHttpRequest) SessionRefreshRequest {
	text, _ := json.Marshal(request.Body)
	body := SessionRefreshRequest{}
	_ = json.Unmarshal(text, &body)
	return body
}
