// ProviderCalendarService reads the caller's authenticated CalDAV resources
// into the ordinary calendar event model and writes changes back through the
// Go owner of external accounts.
// @Service
type ProviderCalendarService struct{}

// Available reports whether provider calendars can be reached.
func (service *ProviderCalendarService) Available() bool {
	return service.calendars().Available()
}

// List answers the caller's provider events.
func (service *ProviderCalendarService) List(token string) ([]map[string]any, error) {
	return service.calendars().List(token)
}

// Calendars answers the caller's provider calendars.
func (service *ProviderCalendarService) Calendars(token string) ([]any, error) {
	return service.calendars().Calendars(token)
}

// Find answers a provider event and its resource, or nil.
func (service *ProviderCalendarService) Find(token string, eventId string) (*ProviderEvent, error) {
	return service.calendars().Find(token, eventId)
}

// Create writes a new event to one of the caller's writable calendars.
func (service *ProviderCalendarService) Create(token string, calendarId string, event map[string]any, idempotencyKey string) (map[string]any, error) {
	return service.calendars().Create(token, calendarId, event, idempotencyKey)
}

// Update writes an edited event (or one edited occurrence) back.
func (service *ProviderCalendarService) Update(token string, change ProviderChange) (map[string]any, error) {
	return service.calendars().Update(token, change)
}

// Delete removes an event, or excludes one occurrence.
func (service *ProviderCalendarService) Delete(token string, eventId string, recurrenceId string, idempotencyKey string) (map[string]any, error) {
	return service.calendars().Delete(token, eventId, recurrenceId, idempotencyKey)
}

func (*ProviderCalendarService) calendars() ProviderCalendars {
	return ProviderCalendars{Accounts: ProviderAccounts{}}
}
