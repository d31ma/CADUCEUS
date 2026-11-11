import "mesail.dev/packages/outbox"

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
	Assert.Equals(0, len(document.PendingEvents))
	Assert.IsTrue(!document.EventQueued)
}

func (test *InboundDocumentServiceTest) TestOutboxFieldsComeFromTheDocument() {
	stored := InboundRaw{PendingEvents: []string{"a"}, DeadEvents: []string{"b"}, OutboxPending: 1, OutboxDead: 1}
	fields := (&InboundDocumentService{}).OutboxFields(stored)
	Assert.Equals("a", fields.PendingEvents[0])
	Assert.Equals("b", fields.DeadEvents[0])
	Assert.Equals(1, fields.OutboxPending)
	Assert.Equals(1, fields.OutboxDead)
}

func (test *InboundDocumentServiceTest) TestEventPatchCarriesTheFieldsAndMarksTheEventQueued() {
	patch := (&InboundDocumentService{}).EventPatch(outbox.Fields{PendingEvents: []string{"a"}, DeadEvents: []string{}, OutboxPending: 1})
	Assert.Equals("a", patch.PendingEvents[0])
	Assert.Equals(1, patch.OutboxPending)
	Assert.Equals(0, patch.OutboxDead)
	Assert.IsTrue(patch.EventQueued)
}

func (test *InboundDocumentServiceTest) TestWireIdHasTheInboundPrefix() {
	Assert.Equals("in_t9", (&InboundDocumentService{}).WireId(InboundRaw{ID: "t9"}))
}

func (test *InboundDocumentServiceTest) TestAcceptedStateIsAccepted() {
	Assert.Equals("accepted", (&InboundDocumentService{}).AcceptedState())
}
