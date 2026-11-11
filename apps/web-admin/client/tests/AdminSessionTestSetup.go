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
	if request.Path == "/api/v1/identity/session/domain" {
		return AdminSessionTestSetup{}.switched(body["domainId"])
	}
	return AdminSessionTestSetup{}.signedIn(body["login"], body["password"])
}

func (AdminSessionTestSetup) refreshed(sessionKey any) (BehaviorValue, error) {
	if sessionKey != "admin-key" {
		return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
	}
	return AdminSessionTestSetup{}.ana("admin-token-2", "example.com", "mesail-admin"), nil
}

func (AdminSessionTestSetup) switched(domainId any) (BehaviorValue, error) {
	if domainId == "gone.test" {
		return nil, &BehaviorError{Code: "storage", Message: "not_found", Status: 404}
	}
	if domainId == "example.org" {
		return AdminSessionTestSetup{}.carl("switched-token", "example.org", "mesail-admin"), nil
	}
	return AdminSessionTestSetup{}.ana("switched-token", "example.com", "mesail-admin"), nil
}

func (AdminSessionTestSetup) signedIn(login any, password any) (BehaviorValue, error) {
	if password != "right" {
		return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
	}
	switch login {
	case "busy":
		return nil, &BehaviorError{Code: "storage", Message: "locked", Status: 429}
	case "ana@partner.test", "finn":
		return nil, &BehaviorError{Code: "storage", Message: "forbidden", Status: 403}
	case "olu":
		return AdminSessionTestSetup{}.session("operator-token", "operator-key", "*", "olu", "Olu Ade", []any{}, "mesail-operator"), nil
	case "bo@example.com":
		bo := []any{map[string]any{"domainId": "example.com", "admin": false}}
		return AdminSessionTestSetup{}.session("member-token", "member-key", "example.com", "bo", "Bo Lee", bo, "mesail-member"), nil
	case "carl":
		return AdminSessionTestSetup{}.carl("carl-token", "example.com", "mesail-member"), nil
	}
	return AdminSessionTestSetup{}.ana("admin-token", "example.com", "mesail-admin"), nil
}

func (AdminSessionTestSetup) ana(token string, domainId string, role string) BehaviorValue {
	domains := []any{map[string]any{"domainId": "example.com", "admin": true}, map[string]any{"domainId": "example.org", "admin": false}}
	return AdminSessionTestSetup{}.session(token, "admin-key", domainId, "ana", "Ana Ruiz", domains, role)
}

func (AdminSessionTestSetup) carl(token string, domainId string, role string) BehaviorValue {
	domains := []any{map[string]any{"domainId": "example.com", "admin": false}, map[string]any{"domainId": "example.org", "admin": true}}
	return AdminSessionTestSetup{}.session(token, "carl-key", domainId, "carl", "Carl Diaz", domains, role)
}

func (AdminSessionTestSetup) session(token string, key string, domainId string, username string, name string, domains []any, role string) BehaviorValue {
	me := map[string]any{"memberId": username + "-id", "domainId": domainId, "username": username, "displayName": name, "primaryAddress": username + "@" + domainId, "roles": []any{role}}
	return map[string]any{"accessToken": token, "expiresIn": 60, "sessionKey": key, "me": me, "domains": domains, "mustChangePassword": false}
}
