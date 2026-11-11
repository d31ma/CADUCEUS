// @TestSetup
type MemberOfflineDirectoryTestSetup struct{}

func (MemberOfflineDirectoryTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	BrowserFakes{}.Answer("clock", "now", 1000)
	config := NewBehaviorConfiguration()
	config.Http["directory"] = MemberOfflineDirectoryTestSetup{}
	config.Http["identity"] = MemberOfflineDirectoryTestSetup{}
	return config, nil
}

func (MemberOfflineDirectoryTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
}
