// @Setup
type AttachmentsSetup struct{}

func (AttachmentsSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
