// @TestSetup
type McpAccessTokensTestSetup struct{}

func (McpAccessTokensTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
