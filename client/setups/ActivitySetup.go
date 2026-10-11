// ActivitySetup gives ActivityClient no adapter, so its requests use the browser
// transport to the page's own server.
// @Setup
type ActivitySetup struct{}

func (ActivitySetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
