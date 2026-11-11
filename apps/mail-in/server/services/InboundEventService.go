import (
	"crypto/rand"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Service
type InboundEventService struct {
	// @Needs
	settings *SettingsService
	// @Needs
	inboundDocumentService *InboundDocumentService
	// @Needs
	eventPayloadService *EventPayloadService
}

func (service *InboundEventService) Stage(stored InboundRaw, envelope InboundEnvelope) (InboundEventPatch, error) {
	settings, err := service.settings.Outbox()
	if err != nil {
		return InboundEventPatch{}, err
	}
	payload := service.eventPayloadService.EmailReceived(service.inboundDocumentService.WireId(stored), stored, envelope)
	staged, err := events.NewEnvelopeFactory(service.producer(), time.Now, rand.Reader).New(events.Types{}.EmailReceived(), stored.DomainId, payload)
	if err != nil {
		return InboundEventPatch{}, err
	}
	fields, err := outbox.NewOutbox(settings).Stage(service.inboundDocumentService.OutboxFields(stored), staged)
	return service.inboundDocumentService.EventPatch(fields), err
}

func (service *InboundEventService) producer() string {
	return "mail-in"
}
