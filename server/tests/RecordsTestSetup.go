// @TestSetup
type RecordsTestSetup struct{}

func (RecordsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
