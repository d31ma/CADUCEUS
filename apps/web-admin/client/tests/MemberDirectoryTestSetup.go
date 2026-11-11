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
	if request.Path == "/api/v1/identity/session/refresh" {
		return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
	}
	if request.Method == "POST" {
		body, _ := request.Body.(map[string]any)
		return MemberDirectoryTestSetup{}.alias(body["address"], body["memberId"])
	}
	return MemberDirectoryTestSetup{}.members(request.Query["query"])
}

func (MemberDirectoryTestSetup) members(query any) (BehaviorValue, error) {
	ana := map[string]any{"memberId": "ana-id", "displayName": "Ana Ruiz", "primaryAddress": "ana@example.com"}
	bo := map[string]any{"memberId": "bo-id", "displayName": "Bo Lee", "primaryAddress": "bo@example.com"}
	olu := map[string]any{"memberId": "olu-id", "displayName": "Olu", "primaryAddress": "olu@example.com"}
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
	}
	return map[string]any{"items": []any{ana, bo}, "nextCursor": ""}, nil
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
