// @Setup
type MessagingSendSetup struct{}

func (MessagingSendSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
