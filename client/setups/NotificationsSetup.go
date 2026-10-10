// @Setup
type NotificationsSetup struct{}

func (NotificationsSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
