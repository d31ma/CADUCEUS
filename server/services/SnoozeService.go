// SnoozeService snoozes whole conversations and returns them when due.
// @Service
type SnoozeService struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Snoozes answers the request's conversation snoozing.
func (service *SnoozeService) Snoozes() Snoozes {
	return DeliveryStores{Fylo: service.fyloHelper}.Snoozes()
}
