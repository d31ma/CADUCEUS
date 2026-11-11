// @Setup
type ChannelsSetup struct{}

func (ChannelsSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
