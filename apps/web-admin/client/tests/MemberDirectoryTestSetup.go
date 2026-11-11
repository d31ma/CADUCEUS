import "strings"

// @TestSetup
type MemberDirectoryTestSetup struct{}

func (MemberDirectoryTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	BrowserFakes{}.Answer("clock", "now", 1000)
	config := NewBehaviorConfiguration()
	config.Http["directory"] = MemberDirectoryTestSetup{}
	config.Http["identity"] = MemberDirectoryTestSetup{}
	return config, nil
}

func (MemberDirectoryTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	body, _ := request.Body.(map[string]any)
	switch {
	case request.Path == "/api/v1/identity/session/refresh" && body["sessionKey"] == "bo-key":
		return MemberDirectoryTestSetup{}.boSession(), nil
	case request.Path == "/api/v1/identity/session/refresh":
		return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
	case request.Path == "/api/v1/directory/aliases":
		return MemberDirectoryTestSetup{}.alias(body["address"], body["memberId"])
	case request.Method == "POST":
		return MemberDirectoryTestSetup{}.added(body["username"], body["displayName"], body["firstPassword"], strings.Contains(request.Path, "/domains/"))
	case request.Method == "PATCH":
		return MemberDirectoryTestSetup{}.changed(strings.TrimPrefix(request.Path, "/api/v1/directory/members/"), body["admin"])
	case request.Method == "DELETE":
		return MemberDirectoryTestSetup{}.removed(strings.TrimPrefix(request.Path, "/api/v1/directory/members/"))
	}
	return MemberDirectoryTestSetup{}.members(request.Query["query"])
}

func (MemberDirectoryTestSetup) boSession() BehaviorValue {
	me := map[string]any{"memberId": "bo-id", "domainId": "example.com", "username": "bo", "displayName": "Bo Lee", "primaryAddress": "bo@example.com", "roles": []any{"mesail-admin"}}
	domains := []any{map[string]any{"domainId": "example.com", "admin": true}}
	return map[string]any{"accessToken": "bo-token", "expiresIn": 60, "sessionKey": "bo-key", "me": me, "domains": domains}
}

func (MemberDirectoryTestSetup) members(query any) (BehaviorValue, error) {
	ana := MemberDirectoryTestSetup{}.member("ana", "Ana Ruiz", true, true)
	bo := MemberDirectoryTestSetup{}.member("bo", "Bo Lee", true, true)
	eve := MemberDirectoryTestSetup{}.member("eve", "Eve Park", false, false)
	olu := MemberDirectoryTestSetup{}.member("olu", "Olu", false, true)
	switch query {
	case "locked":
		return nil, &BehaviorError{Code: "storage", Message: "forbidden", Status: 403}
	case "broken":
		return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
	case "zoe":
		return map[string]any{"items": []any{}, "nextCursor": ""}, nil
	case "olu":
		return map[string]any{"items": []any{olu}, "nextCursor": ""}, nil
	case "many":
		return map[string]any{"items": []any{ana, bo}, "nextCursor": "page-2"}, nil
	case "team":
		return map[string]any{"items": []any{ana, bo, eve}, "nextCursor": ""}, nil
	}
	return map[string]any{"items": []any{ana, bo}, "nextCursor": ""}, nil
}

func (MemberDirectoryTestSetup) member(username string, name string, admin bool, managed bool) map[string]any {
	return map[string]any{"memberId": username + "-id", "username": username, "displayName": name, "primaryAddress": username + "@example.com", "admin": admin, "accountManaged": managed}
}

func (MemberDirectoryTestSetup) added(username any, displayName any, password any, firstAdmin bool) (BehaviorValue, error) {
	switch {
	case username == "sales":
		return nil, &BehaviorError{Code: "storage", Message: "duplicate", Status: 409}
	case username == "locked":
		return nil, &BehaviorError{Code: "storage", Message: "forbidden", Status: 403}
	case username == "offline":
		return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
	case username == "eve":
		return map[string]any{"member": MemberDirectoryTestSetup{}.member("eve", "Eve Park", firstAdmin, false), "existingPerson": true}, nil
	case displayName == nil:
		return nil, &BehaviorError{Code: "storage", Message: "invalid_request", Status: 400}
	case password == "weakpassword1":
		return nil, &BehaviorError{Code: "storage", Message: "password_policy", Status: 400}
	}
	return map[string]any{"member": MemberDirectoryTestSetup{}.member(username.(string), displayName.(string), firstAdmin, true), "existingPerson": false}, nil
}

func (MemberDirectoryTestSetup) changed(memberId string, admin any) (BehaviorValue, error) {
	switch memberId {
	case "bo-id":
		return nil, &BehaviorError{Code: "storage", Message: "last_admin", Status: 409}
	case "eve-id":
		return nil, &BehaviorError{Code: "storage", Message: "forbidden", Status: 403}
	case "offline-id":
		return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
	}
	return MemberDirectoryTestSetup{}.member("dan", "Dan Example", admin == true, true), nil
}

func (MemberDirectoryTestSetup) removed(memberId string) (BehaviorValue, error) {
	switch memberId {
	case "bo-id":
		return nil, &BehaviorError{Code: "storage", Message: "last_admin", Status: 409}
	case "gone-id":
		return nil, &BehaviorError{Code: "storage", Message: "not_found", Status: 404}
	case "offline-id":
		return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
	}
	return nil, nil
}

func (MemberDirectoryTestSetup) alias(address any, memberId any) (BehaviorValue, error) {
	switch address {
	case "taken@example.com":
		return nil, &BehaviorError{Code: "storage", Message: "conflict", Status: 409}
	case "wrong@example.com":
		return nil, &BehaviorError{Code: "storage", Message: "invalid", Status: 422}
	case "rude@example.com":
		return nil, &BehaviorError{Code: "storage", Message: "invalid", Status: 400}
	case "locked@example.com":
		return nil, &BehaviorError{Code: "storage", Message: "forbidden", Status: 403}
	case "offline@example.com":
		return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
	}
	return map[string]any{"address": address, "domainId": "example.com", "kind": "alias", "targetId": memberId}, nil
}
