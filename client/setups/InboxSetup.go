// InboxSetup gives InboxClient no adapter, so its requests use the browser
// transport to the page's own server.
// @Setup
type InboxSetup struct{}

func (InboxSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
