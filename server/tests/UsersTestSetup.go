// @TestSetup
type UsersTestSetup struct{}

func (UsersTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
