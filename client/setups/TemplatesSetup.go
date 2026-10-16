// @Setup
type TemplatesSetup struct{}

func (TemplatesSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
