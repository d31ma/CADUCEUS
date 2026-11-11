// @TestSetup
type AdminShellTestSetup struct{}

func (AdminShellTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
