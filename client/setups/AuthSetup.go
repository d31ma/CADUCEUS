// AuthSetup gives AuthClient no adapter, so its requests use the browser
// transport to the page's own server.
// @Setup
type AuthSetup struct{}

func (AuthSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
