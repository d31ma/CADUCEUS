// @TestSetup
type AdminSessionTestSetup struct{}

func (AdminSessionTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	BrowserFakes{}.Answer("clock", "now", 1000)
	config := NewBehaviorConfiguration()
	config.Http["identity"] = AdminSessionTestSetup{}
	return config, nil
}

func (AdminSessionTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	body, _ := request.Body.(map[string]any)
	if request.Method == "DELETE" {
		return nil, nil
	}
	if request.Path == "/api/v1/identity/session/refresh" {
		return AdminSessionTestSetup{}.refreshed(body["sessionKey"])
	}
	return AdminSessionTestSetup{}.signedIn(body["address"], body["password"])
}

func (AdminSessionTestSetup) refreshed(sessionKey any) (BehaviorValue, error) {
	if sessionKey != "admin-key" {
		return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
	}
	return AdminSessionTestSetup{}.session("admin-token-2", "admin-key", "example.com", "Ana Ruiz", "mesail-admin"), nil
}

func (AdminSessionTestSetup) signedIn(address any, password any) (BehaviorValue, error) {
	if password != "right" {
		return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
	}
	if address == "busy@example.com" {
		return nil, &BehaviorError{Code: "storage", Message: "locked", Status: 429}
	}
	if address == "olu@mesail-operators" {
		return AdminSessionTestSetup{}.session("operator-token", "operator-key", "mesail-operators", "Olu Ade", "mesail-operator"), nil
	}
	if address == "bo@example.com" {
		return AdminSessionTestSetup{}.session("member-token", "member-key", "example.com", "Bo Lee", "mesail-member"), nil
	}
	return AdminSessionTestSetup{}.session("admin-token", "admin-key", "example.com", "Ana Ruiz", "mesail-admin"), nil
}

func (AdminSessionTestSetup) session(token string, key string, domainId string, name string, role string) BehaviorValue {
	me := map[string]any{"memberId": name, "domainId": domainId, "displayName": name, "primaryAddress": name, "roles": []any{role}}
	return map[string]any{"accessToken": token, "expiresIn": 60, "sessionKey": key, "me": me}
}
