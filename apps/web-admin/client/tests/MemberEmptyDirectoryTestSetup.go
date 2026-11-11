// @TestSetup
type MemberEmptyDirectoryTestSetup struct{}

func (MemberEmptyDirectoryTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	BrowserFakes{}.Answer("clock", "now", 1000)
	config := NewBehaviorConfiguration()
	config.Http["directory"] = MemberEmptyDirectoryTestSetup{}
	config.Http["identity"] = MemberEmptyDirectoryTestSetup{}
	return config, nil
}

func (MemberEmptyDirectoryTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	return map[string]any{"items": []any{}, "nextCursor": ""}, nil
}
