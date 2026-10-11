// SendSetup gives SendClient no adapter, so its requests use the browser
// transport to the page's own server.
// @Setup
type SendSetup struct{}

func (SendSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
