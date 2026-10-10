// @TestSetup
type McpAuditEventsTestSetup struct{}

func (McpAuditEventsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
