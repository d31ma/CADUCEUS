// @TestSetup
type DomainRemovingTestSetup struct{}

func (DomainRemovingTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	BrowserFakes{}.Answer("clock", "now", 1000)
	config := NewBehaviorConfiguration()
	config.Http["directory"] = DomainRemovingTestSetup{}
	config.Http["identity"] = DomainRemovingTestSetup{}
	return config, nil
}

func (DomainRemovingTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	if request.Path == "/api/v1/identity/session/refresh" {
		return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
	}
	return []any{
		DomainRemovingTestSetup{}.domain("example.com", "active"),
		DomainRemovingTestSetup{}.domain("gone.example.org", "removing"),
	}, nil
}

func (DomainRemovingTestSetup) domain(name string, state string) map[string]any {
	dns := map[string]any{"mx": "ok", "spf": "ok", "dkim": "ok", "dmarc": "ok"}
	return map[string]any{"domainId": name, "name": name, "state": state, "dkimSelector": "ms2026a", "dns": dns}
}
