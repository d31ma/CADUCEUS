// @TestSetup
type DomainMigrationsTestSetup struct{}

func (DomainMigrationsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
