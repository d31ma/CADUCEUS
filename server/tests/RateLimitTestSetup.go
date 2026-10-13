// @TestSetup
type RateLimitTestSetup struct{}

func (RateLimitTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
