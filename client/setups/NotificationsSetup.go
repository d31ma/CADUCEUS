// NotificationsSetup gives NotificationsClient no adapter, so its requests use the browser
// transport to the page's own server.
// @Setup
type NotificationsSetup struct{}

func (NotificationsSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
