import (
	"cmp"
	"time"

	"mesail.dev/packages/events"
)

// @Service
type NotificationEventsService struct {
	// @Needs
	settings *MemberSettingsStoreService
	// @Needs
	notifications *NotificationStoreService
	// @Needs
	timestamps *TimestampService
	// @Needs
	created *MessageCreatedService
}

func (service *NotificationEventsService) MemberUpserted(envelope events.Envelope) error {
	payload := events.MemberUpserted{}
	return service.profileUnless(service.read(envelope, &payload), envelope.DomainId, &payload)
}

func (service *NotificationEventsService) ReadPositionMoved(envelope events.Envelope) error {
	payload := events.ReadPositionMoved{}
	failure := events.PayloadReader{}.Read(envelope, &payload)
	readAt, invalidTime := service.timestamps.Parse(payload.ReadAt)
	records, failure := service.unreadUnless(cmp.Or(failure, invalidTime), envelope.DomainId, payload)
	return service.markUnless(failure, service.createdUntil(records, readAt))
}

func (service *NotificationEventsService) MessageCreated(envelope events.Envelope) error {
	payload := events.MessageCreated{}
	return service.applyUnless(service.read(envelope, &payload), envelope.DomainId, &payload)
}

func (service *NotificationEventsService) MessageDeleted(envelope events.Envelope) error {
	payload := events.MessageDeleted{}
	failure := events.PayloadReader{}.Read(envelope, &payload)
	records, failure := service.ofMessageUnless(failure, envelope.DomainId, payload.MessageId)
	return service.markUnless(failure, records)
}

func (service *NotificationEventsService) profileUnless(failure error, domainId string, payload *events.MemberUpserted) error {
	if failure != nil {
		return failure
	}
	profile := MemberProfilePatch{DisplayName: payload.DisplayName, PrimaryAddress: payload.PrimaryAddress, State: payload.State}
	return service.settings.PutProfile(domainId, payload.MemberId, profile)
}

func (service *NotificationEventsService) unreadUnless(failure error, domainId string, payload events.ReadPositionMoved) ([]NotificationRecord, error) {
	if failure != nil {
		return nil, failure
	}
	return service.notifications.FindUnreadInConversation(domainId, payload.MemberId, payload.ConversationId, service.markLimit())
}

func (service *NotificationEventsService) ofMessageUnless(failure error, domainId string, messageId string) ([]NotificationRecord, error) {
	if failure != nil {
		return nil, failure
	}
	return service.notifications.FindByMessage(domainId, messageId, service.markLimit())
}

func (service *NotificationEventsService) applyUnless(failure error, domainId string, payload *events.MessageCreated) error {
	if failure != nil {
		return failure
	}
	return service.created.Apply(domainId, *payload)
}

func (service *NotificationEventsService) read(envelope events.Envelope, target any) error {
	return events.PayloadReader{}.Read(envelope, target)
}

func (service *NotificationEventsService) createdUntil(records []NotificationRecord, readAt time.Time) []NotificationRecord {
	kept := []NotificationRecord{}
	for _, record := range records {
		kept = append(kept, service.ifCreatedUntil(record, readAt)...)
	}
	return kept
}

func (service *NotificationEventsService) ifCreatedUntil(record NotificationRecord, readAt time.Time) []NotificationRecord {
	createdAt, failure := service.timestamps.Parse(record.CreatedAt)
	if failure == nil && !createdAt.After(readAt) {
		return []NotificationRecord{record}
	}
	return []NotificationRecord{}
}

func (service *NotificationEventsService) markUnless(failure error, records []NotificationRecord) error {
	if failure != nil {
		return failure
	}
	return service.notifications.MarkAllRead(records, service.timestamps.Now())
}

func (service *NotificationEventsService) markLimit() int {
	return 20
}
