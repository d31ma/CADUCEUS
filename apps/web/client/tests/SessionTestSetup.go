import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// @TestSetup
type SessionTestSetup struct{}

func (setup SessionTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	configuration := NewBehaviorConfiguration()
	configuration.Http["identity"] = setup
	configuration.Http["IdentityClient"] = setup
	configuration.Http["directory"] = setup
	configuration.Http["DirectoryClient"] = setup
	return configuration, nil
}

func (setup SessionTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	switch request.Method + " " + request.Path {
	case "POST /api/v1/identity/session":
		return setup.signIn(setup.signInRequestOf(request))
	case "POST /api/v1/identity/session/refresh":
		return setup.refresh(setup.refreshRequestOf(request).SessionKey)
	case "POST /api/v1/identity/session/domain":
		return setup.switchDomain(setup.bodyOf(request))
	case "DELETE /api/v1/identity/session":
		return nil, nil
	case "POST /api/v1/identity/password":
		return setup.changePassword(fmt.Sprint(request.Headers["Authorization"]), setup.bodyOf(request))
	case "POST /api/v1/identity/totp/setup":
		return setup.setUpTwoStep(setup.bodyOf(request))
	case "POST /api/v1/identity/totp/confirm":
		return setup.confirmTwoStep(setup.bodyOf(request))
	case "GET /api/v1/identity/totp":
		return setup.twoStepStatus(fmt.Sprint(request.Headers["Authorization"]))
	case "DELETE /api/v1/identity/totp":
		return setup.removeTwoStep(setup.bodyOf(request))
	case "PUT /api/v1/directory/me/default-domain":
		return setup.defaultDomain(setup.bodyOf(request))
	}
	return nil, errors.New("no fake answer for " + request.Method + " " + request.Path)
}

func (setup SessionTestSetup) SessionOf(me Me) Session {
	username, _, _ := strings.Cut(me.PrimaryAddress, "@")
	me.Username = username
	return Session{Me: me, Domains: []DomainChoice{{DomainId: me.DomainId, Default: true}}, AccessToken: "access-1", ExpiresIn: 60, SessionKey: "key-1"}
}

func (setup SessionTestSetup) Accounts(me Me) *SessionAccountService {
	accounts := &SessionAccountService{}
	accounts.Apply(setup.SessionOf(me))
	return accounts
}

func (setup SessionTestSetup) Tokens(behaviors *BehaviorContext, sessionKey string, accessToken string, expiresAtMs float64) *SessionTokenService {
	return &SessionTokenService{
		identity: NewIdentityClientWith(behaviors),
		key:      sessionKey,
		tokens:   SessionTokens{AccessToken: accessToken, ExpiresAtMs: expiresAtMs},
	}
}

func (setup SessionTestSetup) Refused(status int) error {
	return &BehaviorError{Code: "storage", Message: "refused", Status: status}
}

func (setup SessionTestSetup) signIn(request SessionSignInRequest) (BehaviorValue, error) {
	switch request.Login + " " + request.Password + " " + request.OneTimeCode {
	case "ana right ", "ana right 123456", "ana@example.com right ":
		return setup.session("example.com", "access-1", "key-1"), nil
	case "ana@example.org right ":
		return setup.session("example.org", "access-org", "key-org"), nil
	case "ana@partner.test right ", "dan right ":
		return nil, setup.Refused(403)
	case "new right ":
		return setup.firstPassword(), nil
	case "op right ":
		return setup.operator(), nil
	case "ana busy ":
		return nil, setup.Refused(429)
	}
	return nil, setup.Refused(401)
}

func (setup SessionTestSetup) refresh(sessionKey string) (BehaviorValue, error) {
	switch sessionKey {
	case "key-1":
		return setup.session("example.com", "access-2", "key-2"), nil
	case "key-org":
		return setup.session("example.org", "access-org-2", "key-org-2"), nil
	case "key-removed", "key-cut":
		return nil, setup.Refused(403)
	}
	return nil, setup.Refused(401)
}

func (setup SessionTestSetup) switchDomain(body map[string]string) (BehaviorValue, error) {
	switch body["sessionKey"] + " " + body["domainId"] {
	case "key-1 example.org", "key-2 example.org":
		return setup.session("example.org", "access-org", "key-org"), nil
	case "key-org example.com", "key-removed ":
		return setup.session("example.com", "access-back", "key-back"), nil
	case "key-1 gone.example":
		return nil, setup.Refused(404)
	case "key-cut ":
		return nil, setup.Refused(403)
	}
	return nil, setup.Refused(401)
}

func (setup SessionTestSetup) session(domainId string, accessToken string, sessionKey string) Session {
	me := Me{MemberId: "m-1", DomainId: domainId, Username: "ana", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@" + domainId, Roles: []string{}}
	domains := []DomainChoice{{DomainId: "example.org"}, {DomainId: "example.com", Admin: true, Default: true}}
	return Session{Me: me, Domains: domains, AccessToken: accessToken, ExpiresIn: 60, SessionKey: sessionKey}
}

func (setup SessionTestSetup) firstPassword() Session {
	session := setup.session("example.com", "access-1", "key-1")
	session.Me.Username = "new"
	session.MustChangePassword = true
	return session
}

func (setup SessionTestSetup) operator() Session {
	me := Me{MemberId: "", DomainId: "*", Username: "op", DisplayName: "Olu Park", PrimaryAddress: "", Roles: []string{"Operator"}}
	return Session{Me: me, Domains: []DomainChoice{}, AccessToken: "access-op", ExpiresIn: 60, SessionKey: "key-op"}
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

func (setup SessionTestSetup) bodyOf(request *BehaviorHttpRequest) map[string]string {
	text, _ := json.Marshal(request.Body)
	body := map[string]string{}
	_ = json.Unmarshal(text, &body)
	return body
}

func (setup SessionTestSetup) defaultDomain(body map[string]string) (BehaviorValue, error) {
	if body["domainId"] == "gone.example" {
		return nil, setup.Refused(404)
	}
	return nil, nil
}

func (setup SessionTestSetup) changePassword(authorization string, body map[string]string) (BehaviorValue, error) {
	if authorization != "Bearer access-1" || body["currentPassword"] != "right" || body["newPassword"] == "weak" {
		return nil, errors.New("refused")
	}
	return setup.session("example.com", "access-new", "key-new"), nil
}

func (setup SessionTestSetup) setUpTwoStep(body map[string]string) (BehaviorValue, error) {
	if body["currentPassword"] != "right" {
		return nil, errors.New("refused")
	}
	return SessionTwoStepSetup{Secret: "JBSWY3DPEHPK3PXP", AppLink: "otpauth://totp/Mesail:ana@example.com", QrImage: "data:image/png;base64,AAAA", SetupKey: "setup-1"}, nil
}

func (setup SessionTestSetup) confirmTwoStep(body map[string]string) (BehaviorValue, error) {
	if body["setupKey"] != "setup-1" || body["code"] != "123456" {
		return nil, errors.New("invalid code")
	}
	return nil, nil
}

func (setup SessionTestSetup) removeTwoStep(body map[string]string) (BehaviorValue, error) {
	if body["currentPassword"] != "right" || body["oneTimeCode"] != "123456" {
		return nil, errors.New("refused")
	}
	return nil, nil
}

func (setup SessionTestSetup) twoStepStatus(authorization string) (BehaviorValue, error) {
	if authorization != "Bearer access-1" {
		return nil, errors.New("unauthenticated")
	}
	return SessionTwoStepStatus{Enabled: true}, nil
}
