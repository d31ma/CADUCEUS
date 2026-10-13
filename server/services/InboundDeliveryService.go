// InboundDeliveryService runs the inbound delivery pipeline; see
// InboundDelivery.
// @Service
type InboundDeliveryService struct {
	// @Needs
	fyloHelper *FyloHelper
	// @Needs
	inboundService *InboundService
	// @Needs
	spamFilterService *SpamFilterService
	// @Needs
	contactsService *ContactsService
	// @Needs
	calendarImportService *CalendarImportService
}

// Delivery answers the request's inbound pipeline.
func (service *InboundDeliveryService) Delivery() InboundDelivery {
	return DeliveryStores{Fylo: service.fyloHelper, Contacts: service.contactsService, Calendar: service.calendarImportService}.Inbound(service.inboundService.Routing(), service.spamFilterService)
}
