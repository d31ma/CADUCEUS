// @TestSetup
type DirectoryTestSetup struct{}

func (DirectoryTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
