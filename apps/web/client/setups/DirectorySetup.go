// @Setup
type DirectorySetup struct{}

func (DirectorySetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
