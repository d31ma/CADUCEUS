// SameOriginSetup gives every remote no adapter, so their requests use the
// browser transport to the page's own server.
// @Setup
type SameOriginSetup struct{}

func (SameOriginSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
