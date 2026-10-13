// @TestSetup
type ScheduledTestSetup struct{}

func (ScheduledTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
