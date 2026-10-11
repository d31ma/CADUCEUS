// ExternalAccountsSetup gives ExternalAccountsClient no adapter, so its
// requests use the browser transport to the page's own server.
// @Setup
type ExternalAccountsSetup struct{}

func (ExternalAccountsSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
