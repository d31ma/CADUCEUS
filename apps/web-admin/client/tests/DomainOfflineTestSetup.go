// @TestSetup
type DomainOfflineTestSetup struct{}

func (DomainOfflineTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	BrowserFakes{}.Answer("clock", "now", 1000)
	config := NewBehaviorConfiguration()
	config.Http["directory"] = DomainOfflineTestSetup{}
	config.Http["identity"] = DomainOfflineTestSetup{}
	return config, nil
}

func (DomainOfflineTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	if request.Path == "/api/v1/identity/session/refresh" {
		return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
	}
	return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
}
