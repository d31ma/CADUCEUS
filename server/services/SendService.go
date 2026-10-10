// SendService accepts outbound mail (now, delayed with undo, or scheduled)
// and keeps its Sent copy.
// @Service
type SendService struct {
	// @Needs
	fyloHelper *FyloHelper
	// @Needs
	smtpService *SmtpService
}

// Sent answers the request's outbound mail acceptance.
func (service *SendService) Sent() SentMail {
	return DeliveryStores{Fylo: service.fyloHelper}.Sent(service.smtpService)
}

// Acceptance answers POST /send's acceptance with the request's journal and
// rate limits.
func (service *SendService) Acceptance(journal SendIdempotency, limits *RateLimitService) SendAcceptance {
	return DeliveryStores{Fylo: service.fyloHelper}.Acceptance(service.smtpService, journal, limits)
}

// Schedules answers scheduled-message management.
func (service *SendService) Schedules() ScheduledSends {
	return DeliveryStores{Fylo: service.fyloHelper}.Schedules(service.smtpService)
}

// Editing answers scheduled-message editing.
func (service *SendService) Editing() ScheduledEditing {
	return DeliveryStores{Fylo: service.fyloHelper}.Editing(service.smtpService)
}

// Undo answers delayed-send undo.
func (service *SendService) Undo() SendUndo {
	return DeliveryStores{Fylo: service.fyloHelper}.Undo(service.smtpService)
}
