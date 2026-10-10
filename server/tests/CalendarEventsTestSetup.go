// @TestSetup
type CalendarEventsTestSetup struct{}

func (CalendarEventsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
