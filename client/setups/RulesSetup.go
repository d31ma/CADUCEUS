// RulesSetup gives RulesClient no adapter, so its requests use the browser
// transport to the page's own server.
// @Setup
type RulesSetup struct{}

func (RulesSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
