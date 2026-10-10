// @Setup
type GrammarSetup struct{}

func (GrammarSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
