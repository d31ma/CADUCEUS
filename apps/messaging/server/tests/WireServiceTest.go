import (
	"encoding/json"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

// @Test(MessagingTestSetup, {"partition":"tests"})
type WireServiceTest struct {
	behaviors *BehaviorContext
}

func (test *WireServiceTest) TestAnEmptyConversationPageHasAnEmptyList() {
	page := (&WireService{}).ConversationPage(core.ConversationList{})
	Assert.Equals(ConversationPage{Items: []ConversationSummary{}}, page)
}

func (test *WireServiceTest) TestAConversationSummaryCopiesTheCounts() {
	view := core.ConversationView{ConversationId: "cnv_1", Title: "Bo", UnreadCount: 2, MentionCount: 1, Conversation: core.Conversation{Kind: "direct", HasExternal: true, LastMessageAt: "2026-11-10T12:00:00Z"}}
	page := (&WireService{}).ConversationPage(core.ConversationList{Items: []core.ConversationView{view}, NextCursor: "next"})
	summary := ConversationSummary{ConversationId: "cnv_1", Kind: "direct", Title: "Bo", HasExternal: true, UnreadCount: 2, MentionCount: 1, LastMessageAt: "2026-11-10T12:00:00Z"}
	Assert.Equals(ConversationPage{Items: []ConversationSummary{summary}, NextCursor: "next"}, page)
}

func (test *WireServiceTest) TestAnEmptyMessagePageHasAnEmptyList() {
	page := (&WireService{}).MessagePage(core.MessageList{})
	Assert.Equals(MessagePage{Items: []Message{}}, page)
}

func (test *WireServiceTest) TestAChatMessageHasNoEmailAndNoNullLists() {
	view := core.MessageView{Message: core.Message{Id: "000000000001", Month: "202611", ConversationId: "cnv_1", AuthorKind: "member", AuthorMemberId: "ana", Body: "hi", BodyFormat: "markdown", CreatedAt: "2026-11-10T12:00:00Z"}}
	message := (&WireService{}).Message(view)
	Assert.Equals("msg_202611000000000001", message.MessageId)
	Assert.IsTrue(message.Email == nil, "a chat message has no email summary")
	Assert.Equals([]Block{}, message.Blocks)
	Assert.Equals([]AttachmentRef{}, message.Attachments)
	Assert.Equals(map[string]int{}, message.Reactions)
	Assert.Equals([]string{}, message.MyReactions)
	Assert.Equals([]DeliveryStatus{}, message.Deliveries)
	Assert.Equals(Author{Kind: "member", MemberId: "ana"}, message.Author)
}

func (test *WireServiceTest) TestAnEmailMessageCarriesItsSummary() {
	email := core.EmailMeta{Subject: "Invoice", From: "carol@partner.org", To: []string{"ana@example.com"}, Suspicious: true, Spf: "pass", Dkim: "fail", Dmarc: "none"}
	view := core.MessageView{Message: core.Message{Id: "000000000002", Month: "202611", IsEmail: true, Email: email, AttachmentIds: []string{"att_1"}}}
	message := (&WireService{}).Message(view)
	summary := EmailSummary{Subject: "Invoice", From: "carol@partner.org", To: []string{"ana@example.com"}, Cc: []string{}, Suspicious: true, AuthResults: AuthResults{Spf: "pass", Dkim: "fail", Dmarc: "none"}}
	Assert.Equals(summary, *message.Email)
	Assert.Equals([]AttachmentRef{{AttachmentId: "att_1"}}, message.Attachments)
}

func (test *WireServiceTest) TestAttachmentsCarryTheirStoredDetails() {
	stored := `[{"attachmentId":"att_1","fileName":"plan.pdf","mediaType":"application/pdf","size":2048}]`
	view := core.MessageView{Message: core.Message{Id: "000000000004", Month: "202611", AttachmentIds: []string{"att_1"}, AttachmentsJson: stored}}
	message := (&WireService{}).Message(view)
	Assert.Equals([]AttachmentRef{{AttachmentId: "att_1", FileName: "plan.pdf", MediaType: "application/pdf", Size: 2048}}, message.Attachments)
}

func (test *WireServiceTest) TestAHandedOffEmailSaysSo() {
	email := core.EmailMeta{Subject: "Hello", From: "eve@example.org", Handoff: true}
	view := core.MessageView{Message: core.Message{Id: "000000000003", Month: "202611", IsEmail: true, Email: email}}
	message := (&WireService{}).Message(view)
	Assert.IsTrue(message.Email.AuthResults.Handoff, "the hand-off flag was dropped")
	Assert.Equals("", message.Email.AuthResults.Spf)
}

func (test *WireServiceTest) TestBlocksKeepSpansAndImages() {
	blocks := []core.Block{
		{Kind: "paragraph", Spans: []core.Span{{Text: "docs", Bold: true, Href: "https://example.com"}}},
		{Kind: "image", Image: &core.ImageRef{Alt: "chart", AttachmentId: "att_2"}},
	}
	message := (&WireService{}).Message(core.MessageView{Blocks: blocks})
	Assert.Equals([]Span{{Text: "docs", Bold: true, Href: "https://example.com"}}, message.Blocks[0].Spans)
	Assert.Equals([]string{}, message.Blocks[0].Cells)
	Assert.IsTrue(message.Blocks[0].Image == nil, "a paragraph has no image")
	Assert.Equals(ImageRef{Alt: "chart", AttachmentId: "att_2"}, *message.Blocks[1].Image)
}

func (test *WireServiceTest) TestSendResultDropsTheMemberAndRole() {
	outcome := core.SendOutcome{Deliveries: []core.DeliveryRecord{{Recipient: "bo@example.com", MemberId: "bo", Channel: "chat", Role: "to", Status: "delivered"}}}
	result := (&WireService{}).SendResult(outcome)
	Assert.Equals([]DeliveryStatus{{Recipient: "bo@example.com", Channel: "chat", Status: "delivered"}}, result.Deliveries)
}

func (test *WireServiceTest) TestSendRequestWithoutEmailHasEmptyEmailOptions() {
	request := SendRequest{ClientMessageId: "c1", Body: "hi", Recipients: []RecipientChoice{{Address: "bo@example.com", Role: "to", Delivery: "chat", ExpectedKind: "member"}}}
	converted := (&WireService{}).SendRequest(request)
	expected := core.SendRequest{ClientMessageId: "c1", Body: "hi", AttachmentIds: []string{}, Recipients: []core.RecipientChoice{{Address: "bo@example.com", Role: "to", Delivery: "chat", ExpectedKind: "member"}}}
	Assert.Equals(expected, converted)
}

func (test *WireServiceTest) TestSendRequestKeepsTheEmailOptions() {
	request := SendRequest{ClientMessageId: "c1", Email: &EmailOptions{Subject: "Hi", FromAddress: "sales@example.com", SendToExternal: true}}
	converted := (&WireService{}).SendRequest(request)
	Assert.Equals(core.EmailOptions{Subject: "Hi", FromAddress: "sales@example.com", SendToExternal: true}, converted.Email)
}

func (test *WireServiceTest) TestAlsoToConversationCrossesTheWireBothWays() {
	converted := (&WireService{}).SendRequest(SendRequest{ClientMessageId: "c1", ThreadRootId: "msg_1", AlsoToConversation: true})
	message := (&WireService{}).Message(core.MessageView{Message: core.Message{ThreadRootId: "msg_1", AlsoToConversation: true}})
	Assert.IsTrue(converted.AlsoToConversation, "the request flag was lost")
	Assert.IsTrue(message.AlsoToConversation, "the message flag was lost")
}

func (test *WireServiceTest) TestMessageSourceNeverHasNullAttachments() {
	source := (&WireService{}).MessageSource(core.MessageSource{MessageId: "msg_1", DomainId: "example.com", Body: "hi"})
	Assert.Equals(MessageSource{MessageId: "msg_1", DomainId: "example.com", Body: "hi", AttachmentIds: []string{}}, source)
}

func (test *WireServiceTest) TestDownloadCopiesTheLink() {
	download := (&WireService{}).Download(core.Download{Url: "https://files.example.com/a.eml", ExpiresAt: "2026-11-10T12:05:00Z"})
	Assert.Equals(Download{Url: "https://files.example.com/a.eml", ExpiresAt: "2026-11-10T12:05:00Z"}, download)
}

func (test *WireServiceTest) TestAnEnvelopeCrossesTheWire() {
	envelope := EventEnvelope{EventId: "evt_1", Type: "member.upserted", DomainId: "a.test", Producer: "directory", OccurredAt: "2026-11-10T12:00:00Z", Payload: json.RawMessage(`{"memberId":"ana"}`)}
	expected := events.Envelope{EventID: "evt_1", Type: "member.upserted", DomainId: "a.test", Producer: "directory", OccurredAt: "2026-11-10T12:00:00Z", Payload: json.RawMessage(`{"memberId":"ana"}`)}
	Assert.Equals(expected, (&WireService{}).Envelope(envelope))
}
