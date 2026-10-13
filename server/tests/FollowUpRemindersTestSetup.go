// @TestSetup
type FollowUpRemindersTestSetup struct{}

func (FollowUpRemindersTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
