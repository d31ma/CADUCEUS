// @Setup
type DomainsSetup struct{}

func (DomainsSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
