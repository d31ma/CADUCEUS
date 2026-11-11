import (
	"cmp"
	"crypto/rand"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Service
type EventService struct {
	// @Needs
	settings *SettingsService
}

func (service *EventService) StageMemberSaved(member MemberDocument) (MemberDocument, error) {
	payload := events.MemberUpserted{MemberId: member.MemberId, DisplayName: member.DisplayName, PrimaryAddress: member.PrimaryAddress, State: member.State}
	envelope, failure := events.NewEnvelopeFactory(service.producer(), time.Now, rand.Reader).New(events.Types{}.MemberUpserted(), member.DomainId, payload)
	settings, _ := service.settings.Outbox()
	fields, stageFailure := outbox.NewOutbox(settings).Stage(service.fields(member), envelope)
	member.PendingEvents, member.DeadEvents, member.OutboxPending, member.OutboxDead = fields.PendingEvents, fields.DeadEvents, fields.OutboxPending, fields.OutboxDead
	return member, cmp.Or(failure, stageFailure)
}

func (service *EventService) fields(member MemberDocument) outbox.Fields {
	return outbox.Fields{PendingEvents: member.PendingEvents, DeadEvents: member.DeadEvents, OutboxPending: member.OutboxPending, OutboxDead: member.OutboxDead}
}

func (service *EventService) producer() string {
	return "directory"
}
