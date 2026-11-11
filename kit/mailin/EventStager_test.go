package mailin

import (
	"testing"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

func TestStageAddsOneEventToTheDocumentFields(t *testing.T) {
	stager := EventStager{Settings: SettingsReader{Environment: goodSettings()}, Now: fixedNow, Random: zeroReader{}}
	stored := InboundRaw{ID: "t1", DomainId: "example.com", Recipients: []string{"ana@example.com"}, PendingEvents: []string{}, DeadEvents: []string{}}
	patch, err := stager.Stage(stored, Envelope{})
	payload := events.EmailReceived{}
	fields := outbox.Fields{PendingEvents: patch.PendingEvents, DeadEvents: patch.DeadEvents, OutboxPending: patch.OutboxPending}
	envelope, one := outbox.Published{}.One(fields, events.Types{}.EmailReceived(), &payload)
	check(t, err == nil && one == nil, "want one event")
	check(t, envelope.DomainId == "example.com" && envelope.Producer == "mail-in", "wrong envelope")
	check(t, payload.InboundId == "in_t1", "wrong payload")
	check(t, patch.OutboxPending == 1 && patch.EventQueued, "event not marked")
	check(t, patch.OutboxWaiting && !patch.OutboxHasDead, "wrong flags")
}

func TestStageReportsMissingSettings(t *testing.T) {
	stager := EventStager{Settings: SettingsReader{Environment: testEnvironment(nil)}, Now: fixedNow, Random: zeroReader{}}
	_, err := stager.Stage(InboundRaw{DomainId: "example.com"}, Envelope{})
	check(t, isUnavailable(err, "outbox settings"), "want outbox settings")
}

func TestStageReportsAnEventThatCannotBeBuilt(t *testing.T) {
	stager := EventStager{Settings: SettingsReader{Environment: goodSettings()}, Now: fixedNow, Random: failingReader{}}
	_, err := stager.Stage(InboundRaw{DomainId: "example.com"}, Envelope{})
	check(t, err != nil, "want an error")
}
