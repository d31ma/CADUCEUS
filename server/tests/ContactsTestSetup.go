// @TestSetup
type ContactsTestSetup struct{}

func (ContactsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
