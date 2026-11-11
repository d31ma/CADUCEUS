import "strings"

// @TestSetup
type PeopleDirectoryTestSetup struct{}

func (PeopleDirectoryTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	BrowserFakes{}.Answer("clock", "now", 1000)
	config := NewBehaviorConfiguration()
	config.Http["directory"] = PeopleDirectoryTestSetup{}
	config.Http["identity"] = PeopleDirectoryTestSetup{}
	return config, nil
}

func (PeopleDirectoryTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	body, _ := request.Body.(map[string]any)
	personId := strings.Split(strings.TrimPrefix(request.Path, "/api/v1/directory/people/"), "/")[0]
	switch {
	case request.Path == "/api/v1/identity/session/refresh":
		return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
	case request.Path == "/api/v1/directory/people":
		return PeopleDirectoryTestSetup{}.people(request.Query["query"])
	case strings.HasSuffix(request.Path, "/password"):
		return PeopleDirectoryTestSetup{}.answer(personId, body["newPassword"] == "weakpassword1")
	}
	return PeopleDirectoryTestSetup{}.answer(personId, false)
}

func (PeopleDirectoryTestSetup) people(query any) (BehaviorValue, error) {
	ana := PeopleDirectoryTestSetup{}.person("ana", "Ana Ruiz", []any{"example.com", "example.org"}, "active")
	dan := PeopleDirectoryTestSetup{}.person("dan", "Dan Example", []any{"example.com"}, "disabled")
	finn := PeopleDirectoryTestSetup{}.person("finn", "Former person", []any{}, "deleted")
	switch query {
	case "locked":
		return nil, &BehaviorError{Code: "storage", Message: "forbidden", Status: 403}
	case "broken":
		return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
	case "zoe":
		return map[string]any{"items": []any{}, "nextCursor": ""}, nil
	case "dan":
		return map[string]any{"items": []any{dan}, "nextCursor": "more"}, nil
	}
	return map[string]any{"items": []any{ana, dan, finn}, "nextCursor": ""}, nil
}

func (PeopleDirectoryTestSetup) person(username string, name string, domains []any, state string) map[string]any {
	return map[string]any{"personId": username + "-id", "username": username, "displayName": name, "domainIds": domains, "state": state}
}

func (PeopleDirectoryTestSetup) answer(personId string, refused bool) (BehaviorValue, error) {
	switch {
	case refused:
		return nil, &BehaviorError{Code: "storage", Message: "password_policy", Status: 400}
	case personId == "eve-id":
		return nil, &BehaviorError{Code: "storage", Message: "forbidden", Status: 403}
	case personId == "offline-id":
		return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
	}
	return PeopleDirectoryTestSetup{}.person("dan", "Dan Example", []any{"example.com"}, "active"), nil
}
