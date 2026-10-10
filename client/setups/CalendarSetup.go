// CalendarSetup gives CalendarClient no adapter, so its requests use the
// browser's same-origin transport.
// @Setup
type CalendarSetup struct{}

func (CalendarSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
