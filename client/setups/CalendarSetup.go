// CalendarSetup gives CalendarClient no adapter, so its requests use the browser
// transport to the page's own server.
// @Setup
type CalendarSetup struct{}

func (CalendarSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
