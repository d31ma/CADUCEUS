// DraftService keeps durable, optimistic-concurrency drafts.
// @Service
type DraftService struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Drafts answers the request's draft lifecycle.
func (service *DraftService) Drafts() Drafts {
	return DeliveryStores{Fylo: service.fyloHelper}.Drafts()
}
