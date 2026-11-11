// @TestSetup
type DomainEmptyTestSetup struct{}

func (DomainEmptyTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	BrowserFakes{}.Answer("clock", "now", 1000)
	config := NewBehaviorConfiguration()
	config.Http["directory"] = DomainEmptyTestSetup{}
	config.Http["identity"] = DomainEmptyTestSetup{}
	return config, nil
}

func (DomainEmptyTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	if request.Path == "/api/v1/identity/session/refresh" {
		return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
	}
	return []any{}, nil
}
