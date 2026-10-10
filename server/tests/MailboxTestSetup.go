// @TestSetup
type MailboxTestSetup struct{}

func (MailboxTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
