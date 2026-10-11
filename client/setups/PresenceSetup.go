// PresenceSetup gives PresenceClient no adapter, so its requests use the browser
// transport to the page's own server.
// @Setup
type PresenceSetup struct{}

func (PresenceSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
