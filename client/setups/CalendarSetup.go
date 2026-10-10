// @Setup
type CalendarSetup struct{}

func (CalendarSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
