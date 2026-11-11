import (
	"os"
	"strings"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Test(InboundTestSetup, {"partition":"tests"})
type OutboxSettingsTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxSettingsTest) TestCollectionsAreNamedWithTheDomainKey() {
	names := &CollectionNamesService{}
	Assert.IsTrue(strings.HasPrefix(names.InboundRaw("example.com"), "inbound-raw-"), "want the inbound-raw base")
	Assert.Equals("outbox-collections", names.Registry())
}

func (test *OutboxSettingsTest) TestSettingsComeFromTheApplicationJson() {
	InboundTestSetup{}.World()
	settings, err := (&SettingsService{}).Outbox()
	Assert.IsTrue(err == nil, "settings missing")
	Assert.Equals("mail-in", settings.Producer)
	Assert.Equals("messaging", settings.Subscribers[0].Name)
	Assert.Equals(12, settings.Retry.MaxAttempts)
	Assert.Equals("http://127.0.0.1:8104/api/v1/messaging/events", settings.Subscribers[0].URL)
}

func (test *OutboxSettingsTest) TestTheServiceTokenComesFromTheEnvironment() {
	os.Setenv("YON_SERVICE_TOKEN", "service-token")
	token, err := (&SettingsService{}).ServiceToken().Token()
	os.Unsetenv("YON_SERVICE_TOKEN")
	Assert.Equals("service-token", token)
	Assert.IsTrue(err == nil, "the token failed")
}

func (test *OutboxSettingsTest) TestThePayloadUsesAddressStringsAndEmptyLists() {
	envelope := InboundEnvelope{
		Headers:     InboundHeaders{From: MailboxAddress{Name: "Dan", Address: "dan@outside.test"}, To: []MailboxAddress{{Name: "Ana", Address: "ana@example.com"}}},
		AuthResults: AuthResults{Spf: "none", Dkim: "none", Dmarc: "none", Handoff: true},
	}
	stored := InboundRaw{Rcpts: []string{"ana@example.com"}, RawBlobKey: "outside.test/x"}
	payload := (&EventPayloadService{}).EmailReceived("in_t1", stored, envelope)
	Assert.Equals("in_t1", payload.InboundId)
	Assert.Equals("dan@outside.test", payload.Headers.From)
	Assert.Equals("ana@example.com", payload.Headers.To[0])
	Assert.IsTrue(payload.Headers.Cc != nil && len(payload.Headers.Cc) == 0, "cc must be an empty list")
	Assert.IsTrue(payload.Headers.References != nil, "references must be a list")
	Assert.Equals("outside.test/x", payload.RawBlobKey)
	Assert.IsTrue(payload.AuthResults.Handoff, "handoff lost")
}

func (test *OutboxSettingsTest) TestStageAddsOneEventToTheDocumentFields() {
	InboundTestSetup{}.World()
	service := &InboundEventService{settings: &SettingsService{}, inboundDocumentService: &InboundDocumentService{}, eventPayloadService: &EventPayloadService{}}
	stored := InboundRaw{ID: "t1", DomainId: "example.com", Rcpts: []string{"ana@example.com"}, PendingEvents: []string{}, DeadEvents: []string{}}
	patch, err := service.Stage(stored, InboundEnvelope{})
	payload := events.EmailReceived{}
	fields := outbox.Fields{PendingEvents: patch.PendingEvents, DeadEvents: patch.DeadEvents, OutboxPending: patch.OutboxPending}
	envelope, one := outbox.Published{}.One(fields, events.Types{}.EmailReceived(), &payload)
	Assert.IsTrue(err == nil, "stage failed")
	Assert.IsTrue(one == nil, "want one event")
	Assert.Equals("example.com", envelope.DomainId)
	Assert.Equals("in_t1", payload.InboundId)
	Assert.Equals(1, patch.OutboxPending)
	Assert.IsTrue(patch.EventQueued, "event not marked")
}
