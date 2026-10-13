// @TestSetup
type PushSubscriptionsTestSetup struct{}

func (PushSubscriptionsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
