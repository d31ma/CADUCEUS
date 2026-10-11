// SnoozeSetup gives SnoozeClient no adapter, so its requests use the browser
// transport to the page's own server.
// @Setup
type SnoozeSetup struct{}

func (SnoozeSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
