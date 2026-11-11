// @TestSetup
type ChannelEmptyTestSetup struct{}

func (ChannelEmptyTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	BrowserFakes{}.Answer("clock", "now", 1000)
	config := NewBehaviorConfiguration()
	config.Http["channels"] = ChannelEmptyTestSetup{}
	config.Http["identity"] = ChannelEmptyTestSetup{}
	return config, nil
}

func (ChannelEmptyTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	if request.Method == "GET" {
		return []any{}, nil
	}
	return nil, &BehaviorError{Code: "storage", Message: "unauthenticated", Status: 401}
}
