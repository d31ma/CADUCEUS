// @TestSetup
type DomainForbiddenTestSetup struct{}

func (DomainForbiddenTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	BrowserFakes{}.Answer("clock", "now", 1000)
	config := NewBehaviorConfiguration()
	config.Http["directory"] = DomainForbiddenTestSetup{}
	config.Http["identity"] = DomainForbiddenTestSetup{}
	return config, nil
}

func (DomainForbiddenTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	if request.Path == "/api/v1/identity/session/refresh" {
		return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
	}
	return nil, &BehaviorError{Code: "storage", Message: "forbidden", Status: 403}
}
