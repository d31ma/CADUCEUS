package inbound

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

func received() string {
	return events.Types{}.EmailReceived()
}

func TestANewEmailToAMemberStartsAnEmailConversation(t *testing.T) {
	scene := NewScene(t)
	scene.world.Inbound.Contents["example.com/in1"] = core.InboundContent{Text: "Hi Ana", BodyFormat: "email", Blocks: []core.Block{{Kind: "paragraph", Spans: []core.Span{{Text: "Hi Ana"}}, Cells: []string{}}}, Truncated: true}
	scene.Must(events.Types{}.MemberUpserted(), "example.com", events.MemberUpserted{MemberId: "ana", DisplayName: "Ana", PrimaryAddress: "ana@example.com", State: "active"})
	mail := Email("in1", "Dan@Outside.test", []string{"ana@example.com"})
	mail.Headers.Cc = []string{"erin@outside.test", "bo@example.com"}
	scene.Must(received(), "example.com", mail)

	conversations := scene.Conversations()
	if len(conversations) != 1 || conversations[0].Kind != "direct" || !conversations[0].HasExternal || conversations[0].Subject != "Hello in1" || conversations[0].MessageCount != 1 || conversations[0].LastMessageAt != "2026-11-10T12:00:00Z" {
		t.Fatalf("got %+v", conversations)
	}
	participants := scene.Participants()
	if len(participants) != 3 {
		t.Fatalf("want dan, ana and erin, got %+v", participants)
	}
	if participants[0].Kind != "external" || participants[0].Address != "dan@outside.test" || participants[0].Role != "to" {
		t.Fatalf("sender: %+v", participants[0])
	}
	if participants[1].Kind != "member" || participants[1].MemberId != "ana" || participants[1].DisplayName != "Ana" || participants[1].Address != "ana@example.com" {
		t.Fatalf("member: %+v", participants[1])
	}
	if participants[2].Kind != "external" || participants[2].Address != "erin@outside.test" || participants[2].Role != "cc" {
		t.Fatalf("cc: %+v", participants[2])
	}
	message := scene.Messages()[0]
	if !message.IsEmail || !message.EventsStaged || message.Sequence != 1 || message.AuthorKind != "external" || message.AuthorAddress != "dan@outside.test" || message.ThreadRootId != "" {
		t.Fatalf("message: %+v", message)
	}
	if message.Body != "Hi Ana" || message.BodyFormat != "email" || !message.Truncated || message.BlocksJson == "[]" || len(message.Deliveries) != 0 {
		t.Fatalf("content: %+v", message)
	}
	if message.Email.InboundId != "in1" || message.Email.Spf != "pass" || message.Email.RawBlobKey != "example.com/inbound-in1" || message.Email.MessageIdHeader != "in1@mail.outside.test" {
		t.Fatalf("meta: %+v", message.Email)
	}
	created := scene.Created(message)
	if !created.IsEmail || created.Subject != "Hello in1" || created.ConversationKind != "direct" || !slices.Equal(created.MemberIds, []string{"ana"}) || created.AuthorKind != "external" || created.MessageId != (core.MessageIds{}).Wire("202611", message.Id) {
		t.Fatalf("event: %+v", created)
	}
	keys := scene.ThreadKeys()
	if len(keys) != 1 || keys[0].Key != (MessageIdKeys{}).Of("<IN1@mail.outside.test>") || keys[0].MessageId != created.MessageId {
		t.Fatalf("thread keys: %+v", keys)
	}
}

func TestAnEmailWithItsContentNeedsNoCallToMailIn(t *testing.T) {
	scene := NewScene(t)
	scene.world.Inbound.Failure = errors.New("mail-in is busy pushing this event")
	mail := Email("in30", "dan@outside.test", []string{"ana@example.com"})
	blocks := `[{"kind":"paragraph","level":0,"ordered":false,"spans":[{"text":"Hi from the event"}],"cells":[]}]`
	attachments := []events.EmailAttachment{{FileName: "a.pdf", MediaType: "application/pdf", Size: 3, BlobKey: "example.com/a"}}
	mail.Content = &events.EmailContent{Text: "Hi from the event", BodyFormat: "email", Blocks: []byte(blocks), Truncated: true, Attachments: attachments}
	scene.Must(received(), "example.com", mail)
	message := scene.Messages()[0]
	if message.Body != "Hi from the event" || message.BodyFormat != "email" || !message.Truncated || message.BlocksJson == "[]" {
		t.Fatalf("content: %+v", message)
	}
}

func TestReceivedContentKeepsTheAttachments(t *testing.T) {
	attachments := []events.EmailAttachment{{FileName: "a.pdf", MediaType: "application/pdf", Size: 3, BlobKey: "example.com/a", ContentId: "a"}}
	content, err := ReceivedContent{}.From(events.EmailContent{Text: "Hi", Blocks: []byte("[]"), Attachments: attachments})
	if err != nil || content.Text != "Hi" || len(content.Blocks) != 0 || content.Attachments[0].BlobKey != "example.com/a" || content.Attachments[0].ContentId != "a" {
		t.Fatalf("got %+v, %v", content, err)
	}
}

func TestReceivedContentRefusesBlocksThatAreNotAList(t *testing.T) {
	_, err := ReceivedContent{}.From(events.EmailContent{Blocks: []byte(`{"kind":"paragraph"}`)})
	if err == nil {
		t.Fatalf("want an error")
	}
}

func TestAnEmailToTwoMembersIsOneSharedConversation(t *testing.T) {
	scene := NewScene(t)
	scene.Must(received(), "example.com", Email("in2", "dan@outside.test", []string{"ana@example.com", "bo@example.com"}))
	if len(scene.Conversations()) != 1 || len(scene.Messages()) != 1 {
		t.Fatalf("want one conversation and one message")
	}
	created := scene.Created(scene.Messages()[0])
	if !slices.Equal(created.MemberIds, []string{"ana", "bo"}) {
		t.Fatalf("got %+v", created.MemberIds)
	}
}

func TestAHandedOverEmailComesFromAnotherDomain(t *testing.T) {
	scene := NewScene(t)
	mail := Email("in3", "eve@example.org", []string{"ana@example.com"})
	mail.AuthResults = events.AuthResults{Handoff: true}
	scene.Must(received(), "example.com", mail)
	if scene.Participants()[0].Kind != "otherDomain" || scene.Messages()[0].AuthorKind != "otherDomain" || !scene.Messages()[0].Email.Handoff {
		t.Fatalf("got %+v", scene.Participants()[0])
	}
}

func TestAnEmailIntoAChannelAddressIsANewRootMessageThere(t *testing.T) {
	scene := NewScene(t)
	scene.Channel("example.com", "chan1")
	scene.Must(events.Types{}.ChannelMemberAdded(), "example.com", events.ChannelMemberAdded{ChannelId: "chan1", ConversationId: "cnv_chan1", MemberId: "bo"})
	scene.Must(received(), "example.com", Email("in4", "customer@outside.test", []string{"support@example.com", "ana@example.com"}))
	if len(scene.Conversations()) != 1 {
		t.Fatalf("want only the channel conversation, got %+v", scene.Conversations())
	}
	message := scene.Messages()[0]
	if message.ConversationId != "cnv_chan1" || message.ThreadRootId != "" || message.Sequence != 1 {
		t.Fatalf("got %+v", message)
	}
	participants := scene.Participants()
	if len(participants) != 2 || participants[1].Address != "customer@outside.test" || participants[1].Kind != "external" {
		t.Fatalf("got %+v", participants)
	}
	created := scene.Created(message)
	if created.ConversationKind != "channel" || !slices.Equal(created.MemberIds, []string{"bo"}) {
		t.Fatalf("got %+v", created)
	}
}

func TestAnEmailIntoAChannelThatIsNotCreatedYetAsksForARetry(t *testing.T) {
	scene := NewScene(t)
	err := scene.Send(received(), "example.com", Email("in5", "customer@outside.test", []string{"support@example.com"}))
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestAReplyToAMessageWeSentJoinsThatConversationAndThread(t *testing.T) {
	scene := NewScene(t)
	conversation, sent := scene.SentMessage("example.com")
	sentId := core.MessageIds{}.Wire(sent.Month, sent.Id)
	mail := Email("in6", "carol@outside.test", []string{"ana@example.com", "bo@example.com"})
	mail.Headers.InReplyTo = sentId + "@example.com"
	scene.Must(received(), "example.com", mail)

	if len(scene.Conversations()) != 1 {
		t.Fatalf("want no new conversation, got %+v", scene.Conversations())
	}
	reply := scene.Messages()[1]
	if reply.ConversationId != (core.ConversationIds{}).Wire(conversation) || reply.ThreadRootId != sentId || reply.Sequence != 2 {
		t.Fatalf("got %+v", reply)
	}
	root := scene.Messages()[0]
	if root.ReplyCount != 1 || root.LastReplyAt != reply.CreatedAt {
		t.Fatalf("root: %+v", root)
	}
	participants := scene.Participants()
	if len(participants) != 3 || participants[2].MemberId != "bo" {
		t.Fatalf("want bo added, got %+v", participants)
	}
	created := scene.Created(reply)
	if created.ThreadRootId != sentId || !slices.Equal(created.MemberIds, []string{"ana", "bo"}) {
		t.Fatalf("got %+v", created)
	}
}

func TestAReplyToAReplyJoinsTheFirstRootThroughReferences(t *testing.T) {
	scene := NewScene(t)
	_, sent := scene.SentMessage("example.com")
	sentId := core.MessageIds{}.Wire(sent.Month, sent.Id)
	first := Email("in7", "carol@outside.test", []string{"ana@example.com"})
	first.Headers.InReplyTo = sentId + "@example.com"
	scene.Must(received(), "example.com", first)
	second := Email("in8", "carol@outside.test", []string{"ana@example.com"})
	second.Headers.References = []string{"unknown@else.test", sentId + "@example.com", "in7@mail.outside.test"}
	scene.Must(received(), "example.com", second)
	messages := scene.Messages()
	if messages[2].ThreadRootId != sentId || messages[0].ReplyCount != 2 || messages[2].Sequence != 3 {
		t.Fatalf("got %+v", messages)
	}
}

func TestAMatchInAnotherDomainNeverJoins(t *testing.T) {
	scene := NewScene(t)
	_, sent := scene.SentMessage("example.org")
	mail := Email("in9", "carol@outside.test", []string{"ana@example.com"})
	mail.Headers.InReplyTo = core.MessageIds{}.Wire(sent.Month, sent.Id) + "@example.org"
	scene.Must(received(), "example.com", mail)
	conversations := scene.Conversations()
	message := scene.Messages()[1]
	if len(conversations) != 2 || message.DomainId != "example.com" || message.ConversationId != (core.ConversationIds{}).Wire(conversations[1]) || message.ThreadRootId != "" {
		t.Fatalf("got %+v %+v", conversations, message)
	}
}

func TestARedeliveredEmailWritesNothingNew(t *testing.T) {
	scene := NewScene(t)
	mail := Email("in10", "dan@outside.test", []string{"ana@example.com"})
	scene.Must(received(), "example.com", mail)
	scene.Must(received(), "example.com", mail)
	if len(scene.Conversations()) != 1 || len(scene.Messages()) != 1 || len(scene.Participants()) != 2 || len(scene.ThreadKeys()) != 1 {
		t.Fatalf("a redelivery wrote again")
	}
}

func TestARedeliveryAfterAnInterruptedWriteFinishesIt(t *testing.T) {
	scene := NewScene(t)
	mail := Email("in11", "dan@outside.test", []string{"ana@example.com"})
	scene.FailAt(13)
	err := scene.Send(received(), "example.com", mail)
	if !errors.As(err, new(*core.Unavailable)) || scene.Messages()[0].EventsStaged {
		t.Fatalf("want an interrupted write, got %v", err)
	}
	scene.FailAt(0)
	scene.Must(received(), "example.com", mail)
	message := scene.Messages()[0]
	if len(scene.Messages()) != 1 || !message.EventsStaged || scene.Conversations()[0].MessageCount != 1 || len(scene.ThreadKeys()) != 1 {
		t.Fatalf("got %+v", message)
	}
	scene.Created(message)
}

func TestARedeliveryLastMonthIsStillFound(t *testing.T) {
	scene := NewScene(t)
	mail := Email("in12", "dan@outside.test", []string{"ana@example.com"})
	scene.Must(received(), "example.com", mail)
	scene.world.Clock.Advance(24 * 25 * 3600 * 1e9)
	scene.Must(received(), "example.com", mail)
	if len(scene.Messages()) != 1 {
		t.Fatalf("a redelivery next month wrote again")
	}
}

func TestANewEmailReportsEveryStoreFailure(t *testing.T) {
	gate := assertNewEmailFails(t, 1)
	assertNewEmailFails(t, 2)
	assertNewEmailFails(t, 3)
	assertNewEmailFails(t, 4)
	assertNewEmailFails(t, 5)
	assertNewEmailFails(t, 6)
	assertNewEmailFails(t, 7)
	assertNewEmailFails(t, 8)
	assertNewEmailFails(t, 9)
	assertNewEmailFails(t, 10)
	assertNewEmailFails(t, 11)
	assertNewEmailFails(t, 12)
	assertNewEmailFails(t, 13)
	assertNewEmailFails(t, 14)
	if gate.calls != 1 {
		t.Fatalf("got %d calls", gate.calls)
	}
}

func assertNewEmailFails(t *testing.T, call int) *Gate {
	t.Helper()
	scene := NewScene(t)
	gate := scene.FailAt(call)
	err := scene.Send(received(), "example.com", Email("in13", "dan@outside.test", []string{"ana@example.com"}))
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("call %d: want Unavailable, got %v", call, err)
	}
	return gate
}

func TestAReplyReportsEveryStoreFailure(t *testing.T) {
	assertReplyFails(t, 3)
	assertReplyFails(t, 4)
	assertReplyFails(t, 5)
	assertReplyFails(t, 6)
	assertReplyFails(t, 7)
	assertReplyFails(t, 12)
}

func assertReplyFails(t *testing.T, call int) {
	t.Helper()
	scene := NewScene(t)
	_, sent := scene.SentMessage("example.com")
	mail := Email("in14", "carol@outside.test", []string{"ana@example.com"})
	mail.Headers.InReplyTo = core.MessageIds{}.Wire(sent.Month, sent.Id) + "@example.com"
	scene.FailAt(call)
	err := scene.Send(received(), "example.com", mail)
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("call %d: want Unavailable, got %v", call, err)
	}
}

func TestAReplyWhoseThreadIsMissingAsksForARetry(t *testing.T) {
	scene := NewScene(t)
	scene.memory.ThreadKeys.Insert(core.ThreadKey{DomainId: "example.com", Key: MessageIdKeys{}.Of("gone@example.com"), ConversationId: "cnv_000000000009", MessageId: "msg_202611000000000009"})
	mail := Email("in15", "carol@outside.test", []string{"ana@example.com"})
	mail.Headers.InReplyTo = "gone@example.com"
	err := scene.Send(received(), "example.com", mail)
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestAReplyWithACorruptThreadKeyIsRefused(t *testing.T) {
	assertCorruptThreadKeyFails(t, "bad", "msg_202611000000000001")
	assertCorruptThreadKeyFails(t, "cnv_000000000001", "bad")
}

func assertCorruptThreadKeyFails(t *testing.T, conversationId string, messageId string) {
	t.Helper()
	scene := NewScene(t)
	scene.SentMessage("example.com")
	scene.memory.ThreadKeys.Insert(core.ThreadKey{DomainId: "example.com", Key: MessageIdKeys{}.Of("odd@example.com"), ConversationId: conversationId, MessageId: messageId})
	mail := Email("in16", "carol@outside.test", []string{"ana@example.com"})
	mail.Headers.InReplyTo = "odd@example.com"
	err := scene.Send(received(), "example.com", mail)
	if !errors.As(err, new(*core.NotFound)) {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestAReplyToAReplyReportsARootFailure(t *testing.T) {
	scene := NewScene(t)
	_, sent := scene.SentMessage("example.com")
	sentId := core.MessageIds{}.Wire(sent.Month, sent.Id)
	first := Email("in17", "carol@outside.test", []string{"ana@example.com"})
	first.Headers.InReplyTo = sentId + "@example.com"
	scene.Must(received(), "example.com", first)
	second := Email("in18", "carol@outside.test", []string{"ana@example.com"})
	second.Headers.InReplyTo = "in17@mail.outside.test"
	scene.FailAt(6)
	err := scene.Send(received(), "example.com", second)
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestAChannelEmailReportsAStoreFailure(t *testing.T) {
	scene := NewScene(t)
	scene.Channel("example.com", "chan1")
	scene.FailAt(4)
	err := scene.Send(received(), "example.com", Email("in19", "customer@outside.test", []string{"support@example.com"}))
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestAnInterruptedWriteWhoseConversationIsGoneAsksForARetry(t *testing.T) {
	scene := NewScene(t)
	mail := Email("in20", "dan@outside.test", []string{"ana@example.com"})
	scene.memory.Messages.Insert(core.Message{DomainId: "example.com", Month: "202611", SendKey: Digests{}.Hex("inbound\nin20"), ConversationId: "cnv_000000000042"})
	err := scene.Send(received(), "example.com", mail)
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestPortFailuresAskForARetry(t *testing.T) {
	scene := NewScene(t)
	scene.world.Inbound.Failure = errors.New("mail-in is down")
	err := scene.Send(received(), "example.com", Email("in21", "dan@outside.test", []string{"ana@example.com"}))
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("want Unavailable, got %v", err)
	}
	scene.world.Inbound.Failure = nil
	scene.world.Directory.Failure = errors.New("directory is down")
	err = scene.Send(received(), "example.com", Email("in21", "dan@outside.test", []string{"ana@example.com"}))
	if !errors.As(err, new(*core.Unavailable)) || len(scene.Messages()) != 0 {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestADirectoryFailureOnAReplyAsksForARetry(t *testing.T) {
	scene := NewScene(t)
	_, sent := scene.SentMessage("example.com")
	mail := Email("in22", "carol@outside.test", []string{"ana@example.com"})
	mail.Headers.InReplyTo = core.MessageIds{}.Wire(sent.Month, sent.Id) + "@example.com"
	scene.world.Directory.Failure = errors.New("directory is down")
	err := scene.Send(received(), "example.com", mail)
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestABadEmailPayloadIsRefused(t *testing.T) {
	scene := NewScene(t)
	err := scene.Send(received(), "example.com", map[string]string{"inboundId": "x", "extra": "y"})
	if err == nil || errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("want a payload error, got %v", err)
	}
}

func TestAnUnknownRecipientIsLeftOut(t *testing.T) {
	scene := NewScene(t)
	scene.Must(received(), "example.com", Email("in23", "dan@outside.test", []string{"ana@example.com", "ghost@example.com"}))
	participants := scene.Participants()
	if len(participants) != 2 || participants[1].MemberId != "ana" {
		t.Fatalf("got %+v", participants)
	}
}

func TestAReplyCopiedToAChannelAddsOnlyMembers(t *testing.T) {
	scene := NewScene(t)
	_, sent := scene.SentMessage("example.com")
	mail := Email("in24", "carol@outside.test", []string{"ana@example.com", "support@example.com"})
	mail.Headers.InReplyTo = core.MessageIds{}.Wire(sent.Month, sent.Id) + "@example.com"
	scene.Must(received(), "example.com", mail)
	if len(scene.Participants()) != 2 || len(scene.Messages()) != 2 {
		t.Fatalf("got %+v", scene.Participants())
	}
}

func storedEmailWith(t *testing.T, inboundId string, text string, blocks string) core.Message {
	t.Helper()
	scene := NewScene(t)
	mail := Email(inboundId, "dan@outside.test", []string{"ana@example.com"})
	mail.Content = &events.EmailContent{Text: text, BodyFormat: "email", Blocks: []byte(blocks)}
	scene.Must(received(), "example.com", mail)
	return scene.Messages()[0]
}

func TestAnEmailWithinTheMessageBudgetIsKeptWhole(t *testing.T) {
	text := strings.Repeat("a", 100000)
	message := storedEmailWith(t, "in40", text, "[]")
	if message.Body != text || message.BodyFormat != "email" || message.Truncated {
		t.Fatalf("the email was cut: %d bytes, %s, %v", len(message.Body), message.BodyFormat, message.Truncated)
	}
}

func TestAnEmailOverTheMessageBudgetKeepsItsFirstSixtyFourKibibytes(t *testing.T) {
	text := strings.Repeat("a", 200000)
	message := storedEmailWith(t, "in41", text, "[]")
	fits := core.MessageBudget{}.Fits(message.Body, message.BlocksJson)
	budget := EmailBudget{}
	if len(message.Body) != budget.PlainTextBytes() || message.BodyFormat != "text" || !message.Truncated || !fits || message.BlocksJson == "[]" {
		t.Fatalf("got %d bytes, %s, %v, fits %v", len(message.Body), message.BodyFormat, message.Truncated, fits)
	}
}

func TestAnEmailOfQuotesOverTheMessageBudgetKeepsItsFirstSixteenKibibytes(t *testing.T) {
	text := strings.Repeat(`"`, 127000)
	message := storedEmailWith(t, "in42", text, "[]")
	fits := core.MessageBudget{}.Fits(message.Body, message.BlocksJson)
	budget := EmailBudget{}
	if len(message.Body) != budget.LastResortTextBytes() || !message.Truncated || !fits {
		t.Fatalf("got %d bytes, %v, fits %v", len(message.Body), message.Truncated, fits)
	}
}

func TestAnEmailCutKeepsWholeCharacters(t *testing.T) {
	text := "a" + strings.Repeat("\U0001F600", 60000)
	message := storedEmailWith(t, "in43", text, "[]")
	budget := EmailBudget{}
	if !utf8.ValidString(message.Body) || !message.Truncated || len(message.Body) > budget.PlainTextBytes() {
		t.Fatalf("got %d bytes, valid %v", len(message.Body), utf8.ValidString(message.Body))
	}
}
