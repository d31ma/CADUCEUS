// @TestSetup
type ChannelOfflineTestSetup struct{}

func (ChannelOfflineTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	BrowserFakes{}.Answer("clock", "now", 1000)
	config := NewBehaviorConfiguration()
	config.Http["channels"] = ChannelOfflineTestSetup{}
	config.Http["identity"] = ChannelOfflineTestSetup{}
	return config, nil
}

func (ChannelOfflineTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	return nil, &BehaviorError{Code: "HTTP_NETWORK", Message: "offline"}
}
