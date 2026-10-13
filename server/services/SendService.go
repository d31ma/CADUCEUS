// SendService accepts outbound mail (now, delayed with undo, or scheduled)
// and keeps its Sent copy.
// @Service
type SendService struct {
	// @Needs
	fyloHelper *FyloHelper
	// @Needs
	smtpService *SmtpService
	// @Needs
	contactsService *ContactsService
	// @Needs
	conversationArchiveService *ConversationArchiveService
}

// Sent answers the request's outbound mail acceptance.
func (service *SendService) Sent() SentMail {
	return service.stores().Sent(service.smtpService)
}

// Acceptance answers POST /send's acceptance with the request's journal and
// rate limits.
func (service *SendService) Acceptance(journal SendIdempotency, limits *RateLimitService) SendAcceptance {
	return service.stores().Acceptance(service.smtpService, journal, limits)
}

// Schedules answers scheduled-message management.
func (service *SendService) Schedules() ScheduledSends {
	return service.stores().Schedules(service.smtpService)
}

// Editing answers scheduled-message editing.
func (service *SendService) Editing() ScheduledEditing {
	return service.stores().Editing(service.smtpService)
}

// Undo answers delayed-send undo.
func (service *SendService) Undo() SendUndo {
	return service.stores().Undo(service.smtpService)
}

func (service *SendService) stores() DeliveryStores {
	return DeliveryStores{Fylo: service.fyloHelper, Contacts: service.contactsService, Archive: service.conversationArchiveService}
}
