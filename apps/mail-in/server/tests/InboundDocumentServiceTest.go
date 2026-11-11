import (
	"encoding/json"
	"strings"
)

// @Test(InboundTestSetup, {"partition":"tests"})
type InboundDocumentServiceTest struct {
	behaviors *BehaviorContext
}

func (test *InboundDocumentServiceTest) envelope() InboundEnvelope {
	return InboundEnvelope{
		EnvelopeFrom:  "dan@outside.test",
		Recipients:    []string{"ana@example.com"},
		RawBlobKey:    "example.com/inbound/abc.eml",
		Size:          42,
		RemoteAddress: "192.0.2.7",
		Helo:          "mail.outside.test",
		Tls:           true,
		Headers:       InboundHeaders{MessageId: "bb-1@outside.test", Subject: "Hello", From: MailboxAddress{Name: "Dan", Address: "dan@outside.test"}},
		AuthResults:   AuthResults{Spf: "pass", Dkim: "none", Dmarc: "fail", DmarcPolicy: "none", Suspicious: true},
	}
}

func (test *InboundDocumentServiceTest) TestNewCopiesTheEnvelope() {
	document := (&InboundDocumentService{}).New("example.com", test.envelope(), "2026-11-10T22:00:00Z")
	Assert.Equals("example.com", document.DomainId)
	Assert.Equals("example.com/inbound/abc.eml", document.RawBlobKey)
	Assert.Equals("bb-1@outside.test", document.MessageId)
	Assert.Equals("dan@outside.test", document.FromAddress)
	Assert.Equals("fail", document.AuthResults.Dmarc)
	Assert.Equals("accepted", document.State)
	Assert.IsTrue(!document.EventQueued)
}

func (test *InboundDocumentServiceTest) TestEventPatchHoldsOneEmailReceivedEvent() {
	service := &InboundDocumentService{}
	stored := service.New("example.com", test.envelope(), "2026-11-10T22:00:00Z")
	stored.ID = "t1"
	patch, err := service.EventPatch(stored, test.envelope(), "2026-11-10T22:00:01Z")
	event := PendingEvent{}
	json.Unmarshal([]byte(patch.PendingEvents[0]), &event)
	Assert.IsTrue(err == nil)
	Assert.IsTrue(patch.EventQueued)
	Assert.Equals("email.received", event.Type)
	Assert.Equals("example.com", event.DomainId)
	Assert.Equals("mail-in", event.Producer)
	Assert.Equals("in_t1", event.Payload.InboundId)
	Assert.Equals("Hello", event.Payload.Headers.Subject)
	Assert.Equals("ana@example.com", strings.Join(event.Payload.Rcpts, ","))
}

func (test *InboundDocumentServiceTest) TestEventIdIsTheSameForTheSameMessage() {
	service := &InboundDocumentService{}
	stored := InboundRaw{ID: "t1", DomainId: "example.com"}
	first, _ := service.EventPatch(stored, test.envelope(), "a")
	second, _ := service.EventPatch(stored, test.envelope(), "b")
	firstEvent, secondEvent := PendingEvent{}, PendingEvent{}
	json.Unmarshal([]byte(first.PendingEvents[0]), &firstEvent)
	json.Unmarshal([]byte(second.PendingEvents[0]), &secondEvent)
	Assert.Equals(firstEvent.EventId, secondEvent.EventId)
}

func (test *InboundDocumentServiceTest) TestWireIdHasTheInboundPrefix() {
	Assert.Equals("in_t9", (&InboundDocumentService{}).WireId(InboundRaw{ID: "t9"}))
}

func (test *InboundDocumentServiceTest) TestAcceptedStateIsAccepted() {
	Assert.Equals("accepted", (&InboundDocumentService{}).AcceptedState())
}
