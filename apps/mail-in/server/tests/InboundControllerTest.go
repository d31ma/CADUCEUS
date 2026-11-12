import (
	"errors"
	"strings"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/mailin"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

// @Test(InboundTestSetup, {"partition":"tests"})
type InboundControllerTest struct {
	behaviors *BehaviorContext
}

func (test *InboundControllerTest) TestPostMessageRefusesAnEnvelopeWithoutRecipients() {
	world := InboundTestSetup{}.World()
	_, err := world.Inbound.PostMessage(InboundEnvelope{RawBlobKey: world.FreshKey(), Size: 1})
	Assert.IsTrue(world.IsInvalid(err), "want InvalidEnvelope")
}

func (test *InboundControllerTest) TestPostMessageRefusesAKeyOfAnotherDomain() {
	world := InboundTestSetup{}.World()
	_, err := world.Inbound.PostMessage(world.Envelope("example.org/inbound-x"))
	Assert.IsTrue(world.IsInvalid(err), "want InvalidEnvelope")
}

func (test *InboundControllerTest) TestPostMessageStoresTheMessageOnce() {
	world := InboundTestSetup{}.World()
	defer world.Cleanup()
	key := world.StoredKey()
	first, err := world.Inbound.PostMessage(world.Envelope(key))
	second, again := world.Inbound.PostMessage(world.Envelope(key))
	stored := world.Stored(key)
	Assert.IsTrue(err == nil, "first accept failed")
	Assert.IsTrue(again == nil, "second accept failed")
	Assert.Equals(first.InboundId, second.InboundId)
	Assert.Equals("in_"+stored.ID, first.InboundId)
	Assert.Equals("accepted", stored.State)
	Assert.Equals("ana@"+world.Domain, stored.Recipients[0])
}

func (test *InboundControllerTest) TestPostMessageStagesOneEmailReceivedEvent() {
	world := InboundTestSetup{}.World()
	defer world.Cleanup()
	world.Pusher.FailFor("messaging", errors.New("connection refused"))
	key := world.StoredKey()
	accepted, err := world.Inbound.PostMessage(world.Envelope(key))
	world.Must(err)
	stored := world.Stored(key)
	payload := events.EmailReceived{}
	envelope, staged := outbox.Published{}.One(stored.OutboxFields(), events.Types{}.EmailReceived(), &payload)
	Assert.IsTrue(staged == nil, "no single staged event")
	Assert.Equals(world.Domain, envelope.DomainId)
	Assert.Equals("mail-in", envelope.Producer)
	Assert.Equals(accepted.InboundId, payload.InboundId)
	Assert.Equals("ana@"+world.Domain, payload.Rcpts[0])
	Assert.Equals(key, payload.RawBlobKey)
	Assert.Equals("dan@outside.test", payload.Headers.From)
	Assert.Equals("ana@"+world.Domain, payload.Headers.To[0])
	Assert.Equals("Hello", payload.Headers.Subject)
	Assert.IsTrue(payload.Headers.Cc != nil && len(payload.Headers.Cc) == 0, "cc must be an empty list")
	Assert.Equals("pass", payload.AuthResults.Dmarc)
	Assert.IsTrue(payload.Content != nil, "the event carries no content")
	Assert.Equals("text", payload.Content.BodyFormat)
	Assert.IsTrue(strings.Contains(payload.Content.Text, "Lunch"), "wrong content text: "+payload.Content.Text)
	Assert.Equals(1, stored.OutboxPending)
	Assert.IsTrue(stored.EventQueued, "event not marked")
}

func (test *InboundControllerTest) TestALargeEmailIsStagedWithItsContentAndClearedAfterTheRelay() {
	world := InboundTestSetup{}.World()
	defer world.Cleanup()
	world.UseServices()
	world.Pusher.FailFor("messaging", errors.New("connection refused"))
	key := world.FreshKey()
	world.WriteBlob(key, world.LargeTextMessage())
	_, err := world.Inbound.PostMessage(world.Envelope(key))
	world.Must(err)
	payload := events.EmailReceived{}
	_, staged := outbox.Published{}.One(world.Stored(key).OutboxFields(), events.Types{}.EmailReceived(), &payload)
	Assert.IsTrue(staged == nil, "no single staged event")
	Assert.IsTrue(len(payload.Content.Text) > 700000, "the large text was not staged")
	world.Pusher.Recover("messaging")
	_, relayed := world.Outbox.PostRelay()
	Assert.IsTrue(relayed == nil, "relay failed")
	Assert.Equals(0, len(world.Stored(key).PendingEvents))
	Assert.Equals(1, len(world.PushedToMessaging()))
}

func (test *InboundControllerTest) TestARetriedMessageStagesNoSecondEvent() {
	world := InboundTestSetup{}.World()
	defer world.Cleanup()
	world.Pusher.FailFor("messaging", errors.New("connection refused"))
	key := world.StoredKey()
	_, first := world.Inbound.PostMessage(world.Envelope(key))
	_, second := world.Inbound.PostMessage(world.Envelope(key))
	staged, err := outbox.Published{}.Staged(world.Stored(key).OutboxFields(), events.Types{}.EmailReceived())
	Assert.IsTrue(first == nil && second == nil, "accept failed")
	Assert.IsTrue(err == nil, "pending events unreadable")
	Assert.Equals(1, len(staged))
	Assert.Equals(1, len(world.Pusher.Attempted()))
}

func (test *InboundControllerTest) TestPostMessagePushesTheEventToMessagingAtOnce() {
	world := InboundTestSetup{}.World()
	defer world.Cleanup()
	key := world.StoredKey()
	accepted, err := world.Inbound.PostMessage(world.Envelope(key))
	world.Must(err)
	pushed := world.PushedToMessaging()
	payload := events.EmailReceived{}
	Assert.Equals(1, len(pushed))
	world.Must(events.PayloadReader{}.Read(pushed[0], &payload))
	Assert.Equals(accepted.InboundId, payload.InboundId)
	Assert.Equals(0, world.Stored(key).OutboxPending)
}

func (test *InboundControllerTest) TestAHandOffIsStoredUnderTheRecipientDomainAfterTheCopy() {
	world := InboundTestSetup{}.World()
	world.UseServices()
	defer world.Cleanup()
	envelope := world.HandOff(world.Sender)
	world.WriteBlob(envelope.RawBlobKey, "raw hand-off")
	first, err := world.Inbound.PostMessage(envelope)
	second, again := world.Inbound.PostMessage(envelope)
	copied := mailin.BlobKeys{}.HandOffCopy(world.Domain, envelope.RawBlobKey).Path()
	Assert.IsTrue(err == nil && again == nil, "hand-off refused")
	Assert.Equals(first.InboundId, second.InboundId)
	Assert.Equals(copied, world.Stored(copied).RawBlobKey)
	Assert.Equals("raw hand-off", world.ReadBlob(copied))
	Assert.Equals(world.Sender+" "+oidc.InstallWide{}.DomainId(), world.Asked()[0])
}

func (test *InboundControllerTest) TestAHandOffFromAnInactiveDomainIsRefused() {
	world := InboundTestSetup{}.World()
	world.UseServices()
	defer world.Cleanup()
	envelope := world.HandOff(world.Retired)
	world.WriteBlob(envelope.RawBlobKey, "raw hand-off")
	_, err := world.Inbound.PostMessage(envelope)
	Assert.IsTrue(world.IsInvalid(err), "want InvalidEnvelope")
	Assert.Equals("envelopeFrom", world.InvalidField(err))
}

func (test *InboundControllerTest) TestGetOriginalSignsADownloadForTheMessageDomain() {
	world := InboundTestSetup{}.World()
	world.UseServices()
	defer world.Cleanup()
	inboundId := world.Accepted(world.TextMessage())
	download, err := world.Inbound.GetOriginal(world.Bearer(world.Domain), inboundId)
	Assert.IsTrue(err == nil, "no download link")
	Assert.IsTrue(strings.HasPrefix(download.Url, "https://mesail.test/blobs/"+world.Domain+"/inbound-"), "wrong link")
	Assert.IsTrue(strings.Contains(download.Url, "message.eml"), "want the file name")
	expiresAt, _ := time.Parse(time.RFC3339, download.ExpiresAt)
	Assert.IsTrue(expiresAt.After(time.Now().Add(4*time.Minute)) && expiresAt.Before(time.Now().Add(6*time.Minute)), "want about five minutes")
}

func (test *InboundControllerTest) TestGetOriginalHidesTheMessageFromAnotherDomain() {
	world := InboundTestSetup{}.World()
	world.UseServices()
	defer world.Cleanup()
	inboundId := world.Accepted(world.TextMessage())
	_, err := world.Inbound.GetOriginal(world.Bearer(world.Other), inboundId)
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}

func (test *InboundControllerTest) TestGetOriginalRefusesABadToken() {
	world := InboundTestSetup{}.World()
	world.UseServices()
	defer world.Cleanup()
	_, err := world.Inbound.GetOriginal("Bearer not-a-token", "in_1")
	Assert.IsTrue(world.IsUnauthenticated(err), "want Unauthenticated")
}

func (test *InboundControllerTest) TestGetContentConvertsAnHtmlEmailWithAnInlineImageAndAPdf() {
	world := InboundTestSetup{}.World()
	world.UseServices()
	defer world.Cleanup()
	inboundId := world.Accepted(world.HtmlMessage())
	content, err := world.Inbound.GetContent(world.Bearer(world.Domain), inboundId)
	Assert.IsTrue(err == nil, "no content")
	Assert.Equals("email", content.BodyFormat)
	Assert.Equals(2, len(content.Attachments))
	Assert.Equals("logo@outside.test", content.Attachments[0].ContentId)
	Assert.Equals(content.Attachments[0].BlobKey, world.ImageOf(content).AttachmentId)
	Assert.Equals("plan.pdf", content.Attachments[1].FileName)
	Assert.Equals("application/pdf", content.Attachments[1].MediaType)
	Assert.Equals("%PDF-1.4\n", world.ReadBlob(content.Attachments[1].BlobKey))
	Assert.Equals("Hello ", content.Blocks[0].Spans[0].Text)
	Assert.IsTrue(content.Blocks[0].Spans[1].Bold, "want the bold span")
}

func (test *InboundControllerTest) TestGetContentConvertsAPlainTextEmail() {
	world := InboundTestSetup{}.World()
	world.UseServices()
	defer world.Cleanup()
	inboundId := world.Accepted(world.TextMessage())
	content, err := world.Inbound.GetContent(world.Bearer(world.Domain), inboundId)
	Assert.IsTrue(err == nil, "no content")
	Assert.Equals("text", content.BodyFormat)
	Assert.Equals("Lunch at noon?", content.Blocks[0].Spans[0].Text)
	Assert.Equals(0, len(content.Attachments))
}

func (test *InboundControllerTest) TestASecondGetContentReadsTheStoredResult() {
	world := InboundTestSetup{}.World()
	world.UseServices()
	defer world.Cleanup()
	inboundId := world.Accepted(world.HtmlMessage())
	first, err := world.Inbound.GetContent(world.Bearer(world.Domain), inboundId)
	world.Must(err)
	stored, err := world.Workflows.Messages().Book.Find(world.Domain, inboundId)
	world.Must(err)
	world.DeleteBlob(stored.RawBlobKey)
	second, again := world.Inbound.GetContent(world.Bearer(world.Domain), inboundId)
	Assert.IsTrue(again == nil, "the second call read the raw message")
	Assert.Equals(len(first.Blocks), len(second.Blocks))
	Assert.Equals(first.Attachments[1].BlobKey, second.Attachments[1].BlobKey)
}

func (test *InboundControllerTest) TestGetContentHidesTheMessageFromAnotherDomain() {
	world := InboundTestSetup{}.World()
	world.UseServices()
	defer world.Cleanup()
	inboundId := world.Accepted(world.TextMessage())
	_, err := world.Inbound.GetContent(world.Bearer(world.Other), inboundId)
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}
