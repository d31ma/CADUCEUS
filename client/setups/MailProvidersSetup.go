// MailProvidersSetup gives MailProvidersClient no adapter, so its requests
// use the browser transport to the page's own server.
// @Setup
type MailProvidersSetup struct{}

func (MailProvidersSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
