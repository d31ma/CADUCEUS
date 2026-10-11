// UsersSetup gives UsersClient no adapter, so its requests use the browser
// transport to the page's own server.
// @Setup
type UsersSetup struct{}

func (UsersSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
