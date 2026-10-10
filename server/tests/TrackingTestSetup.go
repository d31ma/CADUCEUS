// @TestSetup
type TrackingTestSetup struct{}

func (TrackingTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
