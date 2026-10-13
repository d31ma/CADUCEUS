// @TestSetup
type DomainsTestSetup struct{}

func (DomainsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
