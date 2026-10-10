// @Setup
type ActivitySetup struct{}

func (ActivitySetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
