import "strings"

// @TestSetup
type DomainDirectoryTestSetup struct{}

func (DomainDirectoryTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	BrowserFakes{}.Answer("clock", "now", 1000)
	BrowserFakes{}.Answer("clipboard", "writeText", nil)
	config := NewBehaviorConfiguration()
	config.Http["directory"] = DomainDirectoryTestSetup{}
	config.Http["identity"] = DomainDirectoryTestSetup{}
	return config, nil
}

func (DomainDirectoryTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	if request.Path == "/api/v1/identity/session/refresh" {
		return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
	}
	if request.Method == "GET" {
		return DomainDirectoryTestSetup{}.domains(), nil
	}
	if request.Method == "DELETE" {
		return DomainDirectoryTestSetup{}.removed(request.Path)
	}
	if strings.HasSuffix(request.Path, "/verify") {
		return DomainDirectoryTestSetup{}.verified(request.Path)
	}
	body, _ := request.Body.(map[string]any)
	return DomainDirectoryTestSetup{}.created(body["name"])
}

func (DomainDirectoryTestSetup) domains() BehaviorValue {
	return []any{
		DomainDirectoryTestSetup{}.domain("example.com", "active", "ok", "ok", "ok", "ok"),
		DomainDirectoryTestSetup{}.domain("new.example.net", "pending", "ok", "ok", "missing", "wrong"),
		DomainDirectoryTestSetup{}.domain("old.example.com", "disabled", "", "", "", ""),
	}
}

func (DomainDirectoryTestSetup) removed(path string) (BehaviorValue, error) {
	switch path {
	case "/api/v1/directory/domains/old.example.com":
		return nil, &BehaviorError{Code: "storage", Message: "forbidden", Status: 403}
	case "/api/v1/directory/domains/gone.example":
		return nil, &BehaviorError{Code: "storage", Message: "not found", Status: 404}
	case "/api/v1/directory/domains/new.example.net":
		return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
	}
	return nil, nil
}

func (DomainDirectoryTestSetup) verified(path string) (BehaviorValue, error) {
	switch path {
	case "/api/v1/directory/domains/old.example.com/verify":
		return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
	case "/api/v1/directory/domains/example.com/verify":
		return nil, &BehaviorError{Code: "storage", Message: "forbidden", Status: 403}
	}
	return DomainDirectoryTestSetup{}.domain("new.example.net", "active", "ok", "ok", "ok", "ok"), nil
}

func (DomainDirectoryTestSetup) created(name any) (BehaviorValue, error) {
	switch name {
	case "taken.example":
		return nil, &BehaviorError{Code: "storage", Message: "conflict", Status: 409}
	case "blocked.example":
		return nil, &BehaviorError{Code: "storage", Message: "forbidden", Status: 403}
	case "rude.example":
		return nil, &BehaviorError{Code: "storage", Message: "invalid", Status: 400}
	case "offline.example":
		return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
	}
	return DomainDirectoryTestSetup{}.domain(name.(string), "pending", "", "", "", ""), nil
}

func (DomainDirectoryTestSetup) domain(name string, state string, mx string, spf string, dkim string, dmarc string) map[string]any {
	dns := map[string]any{"mx": mx, "spf": spf, "dkim": dkim, "dmarc": dmarc}
	return map[string]any{"domainId": name, "name": name, "state": state, "dkimSelector": "ms2026a", "dns": dns}
}
