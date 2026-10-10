// ThreadSharesSetup gives ThreadSharesClient no adapter, so its requests use the browser
// transport to the page's own server.
// @Setup
type ThreadSharesSetup struct{}

func (ThreadSharesSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
