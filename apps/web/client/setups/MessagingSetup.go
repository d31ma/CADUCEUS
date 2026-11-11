// @Setup
type MessagingSetup struct{}

func (MessagingSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
