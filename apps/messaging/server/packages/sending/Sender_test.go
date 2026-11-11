package sending

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"mesail.dev/packages/core"
)

func TestAChatToAColleagueStartsADirectConversation(t *testing.T) {
	s := newScene(t)
	outcome := s.mustSend(t, request("c1", "hello Bo", to("Bo@Example.com ", "chat", "member")))
	message := s.stored(t, outcome)
	if got := rowsText(outcome.Deliveries); got != "bo@example.com/chat/to/delivered;" {
		t.Fatalf("deliveries %s", got)
	}
	if message.IsEmail || len(emailRequests(t, message)) != 0 {
		t.Fatalf("a chat must not send email: %+v", message.Email)
	}
	conversations := s.conversations()
	if len(conversations) != 1 || conversations[0].Kind != "direct" || conversations[0].MessageCount != 1 || conversations[0].ParticipantKey == "" {
		t.Fatalf("conversation %+v", conversations)
	}
	if message.Sequence != 1 || message.BodyFormat != "markdown" || message.AuthorAddress != "ana@example.com" || !message.EventsStaged {
		t.Fatalf("message %+v", message)
	}
	payload := created(t, message)
	if payload.ConversationKind != "direct" || !slices.Equal(payload.MemberIds, []string{"bo", "ana"}) || payload.MessageId != wireOf(message) || payload.FileNames == nil {
		t.Fatalf("message.created %+v", payload)
	}
	if s.world.Directory.Authorization[0] != "Bearer ana-token" {
		t.Fatalf("the caller's token must reach the directory: %v", s.world.Directory.Authorization)
	}
	if len(outcome.Message.Blocks) != 1 || outcome.Message.Reactions == nil || outcome.Message.MyReactions == nil {
		t.Fatalf("view %+v", outcome.Message)
	}
}

func TestTheSenderReadPositionMovesToTheNewMessage(t *testing.T) {
	s := newScene(t)
	outcome := s.mustSend(t, request("c1", "hi", to("bo@example.com", "chat", "member")))
	for _, participant := range s.participants() {
		if participant.MemberId == "ana" && (participant.ReadSequence != 1 || participant.ReadMessageId != wireOf(outcome.Message.Message) || participant.ReadAt == "") {
			t.Fatalf("sender position %+v", participant)
		}
		if participant.MemberId == "bo" && participant.ReadSequence != 0 {
			t.Fatalf("recipient position moved %+v", participant)
		}
	}
	second := s.mustSend(t, request("c2", "again", to("bo@example.com", "chat", "member")))
	if second.Message.Message.Sequence != 2 {
		t.Fatalf("second sequence %d", second.Message.Message.Sequence)
	}
	for _, participant := range s.participants() {
		if participant.MemberId == "ana" && participant.ReadSequence != 2 {
			t.Fatalf("sender position not moved %+v", participant)
		}
	}
	if len(s.participants()) != 2 {
		t.Fatalf("participants %+v", s.participants())
	}
}

func TestADirectChatReusesTheConversationOfTheSameMembers(t *testing.T) {
	s := newScene(t)
	first := s.mustSend(t, request("c1", "one", to("bo@example.com", "chat", "member")))
	second := s.mustSend(t, request("c2", "two", to("bo@example.com", "chat", "member")))
	third := s.mustSend(t, request("c3", "three", to("bo@example.com", "chat", "member"), to("cy@example.com", "chat", "member")))
	if first.Message.Message.ConversationId != second.Message.Message.ConversationId {
		t.Fatal("the DM was not reused")
	}
	if third.Message.Message.ConversationId == first.Message.Message.ConversationId {
		t.Fatal("a group must get its own conversation")
	}
}

func TestAChatToMyselfStartsAConversationWithOnlyMe(t *testing.T) {
	s := newScene(t)
	outcome := s.mustSend(t, request("c1", "note", to("ana@example.com", "chat", "member")))
	if len(s.participants()) != 1 || s.participants()[0].ReadSequence != 1 {
		t.Fatalf("participants %+v", s.participants())
	}
	if rowsText(outcome.Deliveries) != "ana@example.com/chat/to/delivered;" {
		t.Fatalf("rows %s", rowsText(outcome.Deliveries))
	}
}

func TestAnEmailToAnOutsidePersonStartsAnEmailConversation(t *testing.T) {
	s := newScene(t)
	sendRequest := request("c1", "Hello Carol", to("carol@partner.org", "email", "external"))
	sendRequest.Email.Subject = "Contract"
	outcome := s.mustSend(t, sendRequest)
	message := s.stored(t, outcome)
	conversation := s.conversations()[0]
	if !conversation.HasExternal || conversation.Subject != "Contract" || conversation.ParticipantKey != "" {
		t.Fatalf("conversation %+v", conversation)
	}
	if rowsText(outcome.Deliveries) != "carol@partner.org/email/to/queued;" {
		t.Fatalf("rows %s", rowsText(outcome.Deliveries))
	}
	header := wireOf(message) + "@example.com"
	if !message.IsEmail || message.Email.MessageIdHeader != header || message.Email.From != "ana@example.com" || message.Email.Subject != "Contract" || message.Email.InReplyTo != "" || len(message.Email.References) != 0 {
		t.Fatalf("email %+v", message.Email)
	}
	requests := emailRequests(t, message)
	if len(requests) != 1 || requests[0].Rcpt != "carol@partner.org" || requests[0].Role != "to" || requests[0].IdempotencyKey != wireOf(message)+"|carol@partner.org" || !slices.Equal(requests[0].To, []string{"carol@partner.org"}) {
		t.Fatalf("email.send.requested %+v", requests)
	}
	keys := s.threadKeys()
	if len(keys) != 1 || keys[0].Key != (Hashes{}).hex(strings.ToLower(header)) || keys[0].MessageId != wireOf(message) {
		t.Fatalf("thread keys %+v", keys)
	}
	external := s.participants()[0]
	if external.Kind != "external" || external.Address != "carol@partner.org" || external.MemberId != "" {
		t.Fatalf("participant %+v", external)
	}
	if !created(t, message).IsEmail || created(t, message).Subject != "Contract" {
		t.Fatalf("message.created %+v", created(t, message))
	}
}

func TestEveryEmailStartsANewConversation(t *testing.T) {
	s := newScene(t)
	first := request("c1", "a", to("carol@partner.org", "email", "external"))
	first.Email.Subject = "One"
	second := request("c2", "b", to("carol@partner.org", "email", "external"))
	second.Email.Subject = "Two"
	s.mustSend(t, first)
	s.mustSend(t, second)
	if len(s.conversations()) != 2 {
		t.Fatalf("conversations %+v", s.conversations())
	}
}

func TestMixedRecipientsGetChatAndEmailFromOneSend(t *testing.T) {
	s := newScene(t)
	sendRequest := request("c1", "Kickoff", to("bo@example.com", "chat", "member"), to("carol@partner.org", "email", "external"), to("eve@example.org", "email", "otherDomain"))
	sendRequest.Email.Subject = "Kickoff"
	outcome := s.mustSend(t, sendRequest)
	message := s.stored(t, outcome)
	if got := rowsText(outcome.Deliveries); got != "bo@example.com/chat/to/delivered;carol@partner.org/email/to/queued;eve@example.org/email/to/queued;" {
		t.Fatalf("rows %s", got)
	}
	if !slices.Equal(message.Email.To, []string{"carol@partner.org", "eve@example.org"}) || !slices.Equal(message.Email.Cc, []string{"bo@example.com"}) {
		t.Fatalf("headers to=%v cc=%v", message.Email.To, message.Email.Cc)
	}
	if len(emailRequests(t, message)) != 2 {
		t.Fatalf("want two emails, got %+v", emailRequests(t, message))
	}
	kinds := []string{}
	for _, participant := range s.participants() {
		kinds = append(kinds, participant.Kind+":"+participant.Address)
	}
	if !slices.Equal(kinds, []string{"member:bo@example.com", "external:carol@partner.org", "otherDomain:eve@example.org", "member:ana@example.com"}) {
		t.Fatalf("participants %v", kinds)
	}
}

func TestAMemberCanGetBothChatAndEmail(t *testing.T) {
	s := newScene(t)
	sendRequest := request("c1", "records", to("bo@example.com", "both", "member"))
	sendRequest.Email.Subject = "Records"
	outcome := s.mustSend(t, sendRequest)
	message := s.stored(t, outcome)
	if got := rowsText(outcome.Deliveries); got != "bo@example.com/chat/to/delivered;bo@example.com/email/to/queued;" {
		t.Fatalf("rows %s", got)
	}
	if !slices.Equal(message.Email.To, []string{"bo@example.com"}) || len(message.Email.Cc) != 0 || s.conversations()[0].HasExternal {
		t.Fatalf("email %+v", message.Email)
	}
}

func TestAnEmailNeedsASubject(t *testing.T) {
	s := newScene(t)
	_, err := s.send(request("c1", "hi", to("carol@partner.org", "email", "external")))
	wantInvalid(t, err, "subject")
	if len(s.messages()) != 0 || len(s.conversations()) != 0 {
		t.Fatal("nothing may be written")
	}
}

func TestAStaleKindAnswersRecipientsChanged(t *testing.T) {
	s := newScene(t)
	_, err := s.send(request("c1", "hi", to("carol@partner.org", "chat", "member"), to("bo@example.com", "chat", "member"), to("eve@example.org", "chat", "member")))
	changed := &core.RecipientsChanged{}
	if !errors.As(err, &changed) || !slices.Equal(changed.Addresses, []string{"carol@partner.org", "eve@example.org"}) {
		t.Fatalf("want RecipientsChanged, got %#v", err)
	}
	if len(s.messages()) != 0 || len(s.conversations()) != 0 {
		t.Fatal("nothing may be written")
	}
}

func TestAnAddressMissingFromTheAnswerAnswersRecipientsChanged(t *testing.T) {
	s := newScene(t)
	s.sender = NewSender(s.world.Stores, portsWith(s, silentDirectory{s.world.Directory}))
	_, err := s.send(request("c1", "hi", to("bo@example.com", "chat", "member")))
	if !errors.As(err, new(*core.RecipientsChanged)) {
		t.Fatalf("want RecipientsChanged, got %#v", err)
	}
}

func TestChatToAnOutsideAddressIsNotAllowed(t *testing.T) {
	cases := []core.RecipientChoice{
		to("carol@partner.org", "chat", "external"),
		to("carol@partner.org", "both", "external"),
		to("eve@example.org", "chat", "otherDomain"),
	}
	for _, recipient := range cases {
		s := newScene(t)
		_, err := s.send(request("c1", "hi", recipient))
		notAllowed := &core.ChatNotAllowed{}
		if !errors.As(err, &notAllowed) || notAllowed.Address != recipient.Address {
			t.Fatalf("%s: want ChatNotAllowed, got %#v", recipient.Address, err)
		}
		if len(s.messages()) != 0 {
			t.Fatal("nothing may be written")
		}
	}
}

func TestEmailToAChannelIsInvalid(t *testing.T) {
	s := newScene(t)
	s.channel(t, false)
	_, err := s.send(request("c1", "hi", to("releases@example.com", "email", "channel")))
	wantInvalid(t, err, "recipients")
}

func TestAChannelMustBeTheOnlyRecipientInTo(t *testing.T) {
	cases := [][]core.RecipientChoice{
		{to("releases@example.com", "chat", "channel"), to("bo@example.com", "chat", "member")},
		{choice("releases@example.com", "cc", "chat", "channel")},
	}
	for _, recipients := range cases {
		s := newScene(t)
		s.channel(t, false)
		_, err := s.send(request("c1", "hi", recipients...))
		wantInvalid(t, err, "recipients")
	}
}

func TestAMessageToAChannelRecipientPostsInTheChannel(t *testing.T) {
	s := newScene(t)
	s.channel(t, false)
	outcome := s.mustSend(t, request("c1", "Build 42 is green", to("releases@example.com", "chat", "channel")))
	message := s.stored(t, outcome)
	if message.ConversationId != "cnv_chan1" || rowsText(outcome.Deliveries) != "releases@example.com/chat/to/delivered;" {
		t.Fatalf("message %s rows %s", message.ConversationId, rowsText(outcome.Deliveries))
	}
	payload := created(t, message)
	if payload.ConversationKind != "channel" || !slices.Equal(payload.MemberIds, []string{"ana", "bo"}) {
		t.Fatalf("message.created %+v", payload)
	}
	if len(s.participants()) != 2 || len(s.conversations()) != 1 || s.conversations()[0].MessageCount != 1 {
		t.Fatalf("channel state changed: %+v %+v", s.participants(), s.conversations())
	}
}

func TestAChannelThatMessagingDoesNotKnowIsNotFound(t *testing.T) {
	s := newScene(t)
	_, err := s.send(request("c1", "hi", to("releases@example.com", "chat", "channel")))
	wantNotFound(t, err)
}

func TestOnlyChannelMembersPostInTheChannel(t *testing.T) {
	s := newScene(t)
	s.channel(t, false)
	cy := s.ana
	cy.MemberId = "cy"
	_, err := s.sender.Send(cy, "Bearer cy", request("c1", "hi", to("releases@example.com", "chat", "channel")))
	wantNotFound(t, err)
}

func TestAnArchivedChannelRefusesPosts(t *testing.T) {
	s := newScene(t)
	s.channel(t, true)
	sendRequest := request("c1", "hi")
	sendRequest.ConversationId = "cnv_chan1"
	_, err := s.send(sendRequest)
	if !errors.As(err, new(*core.Conflict)) {
		t.Fatalf("want Conflict, got %#v", err)
	}
}

func TestAReplyInAnEmailThreadReachesTheCustomerOnlyWhenAsked(t *testing.T) {
	s := newScene(t)
	s.channel(t, false)
	inbound := s.inboundEmail(t)
	internal := request("c1", "looking", to("unused@example.com", "chat", "member"))
	internal.Recipients = nil
	internal.ConversationId = "cnv_chan1"
	internal.ThreadRootId = wireOf(inbound)
	outcome := s.mustSend(t, internal)
	if len(outcome.Deliveries) != 0 || outcome.Message.Message.IsEmail {
		t.Fatalf("an internal reply sent %s", rowsText(outcome.Deliveries))
	}
	external := internal
	external.ClientMessageId = "c2"
	external.Email.SendToExternal = true
	outcome = s.mustSend(t, external)
	message := s.stored(t, outcome)
	if rowsText(outcome.Deliveries) != "dan@partner.org/email/to/queued;" {
		t.Fatalf("rows %s", rowsText(outcome.Deliveries))
	}
	email := message.Email
	if email.From != "support@example.com" || email.Subject != "Re: Printer broken" || email.InReplyTo != "abc@partner.org" || !slices.Equal(email.References, []string{"r1@partner.org", "abc@partner.org"}) {
		t.Fatalf("email %+v", email)
	}
	if message.ThreadRootId != wireOf(inbound) || message.Sequence != 3 {
		t.Fatalf("thread %s sequence %d", message.ThreadRootId, message.Sequence)
	}
	root, _, _ := s.world.Stores.Messages.Find("example.com", inbound.Month, inbound.Id)
	if root.ReplyCount != 2 || root.LastReplyAt == "" {
		t.Fatalf("root %+v", root)
	}
}

func TestAReplyToAReplyAttachesToTheRoot(t *testing.T) {
	s := newScene(t)
	first := s.mustSend(t, request("c1", "root", to("bo@example.com", "chat", "member")))
	reply := request("c2", "reply")
	reply.Recipients = nil
	reply.ConversationId = first.Message.Message.ConversationId
	reply.ThreadRootId = wireOf(first.Message.Message)
	second := s.mustSend(t, reply)
	nested := reply
	nested.ClientMessageId = "c3"
	nested.ThreadRootId = wireOf(second.Message.Message)
	third := s.mustSend(t, nested)
	if third.Message.Message.ThreadRootId != wireOf(first.Message.Message) {
		t.Fatalf("nested reply root %s", third.Message.Message.ThreadRootId)
	}
	root := s.stored(t, first)
	if root.ReplyCount != 2 || root.LastReplyAt == "" {
		t.Fatalf("root %+v", root)
	}
	if rowsText(third.Deliveries) != "bo@example.com/chat/to/delivered;" {
		t.Fatalf("rows %s", rowsText(third.Deliveries))
	}
}

func TestAThreadRootOutsideTheConversationIsNotFound(t *testing.T) {
	s := newScene(t)
	other := s.mustSend(t, request("c1", "with cy", to("cy@example.com", "chat", "member")))
	first := s.mustSend(t, request("c2", "with bo", to("bo@example.com", "chat", "member")))
	reply := request("c3", "reply")
	reply.Recipients = nil
	reply.ConversationId = first.Message.Message.ConversationId
	reply.ThreadRootId = wireOf(other.Message.Message)
	_, err := s.send(reply)
	wantNotFound(t, err)
	reply.ThreadRootId = "msg_202611999999999999"
	_, err = s.send(reply)
	wantNotFound(t, err)
	orphan := core.Message{DomainId: "example.com", ConversationId: first.Message.Message.ConversationId, Month: "202611", ThreadRootId: "msg_202611999999999999"}
	orphanId, _ := s.world.Stores.Messages.Insert(orphan)
	reply.ThreadRootId = "msg_202611" + orphanId
	_, err = s.send(reply)
	wantNotFound(t, err)
	reply.ThreadRootId = "nonsense"
	_, err = s.send(reply)
	wantNotFound(t, err)
}

func TestAnotherDomainsConversationIsNotFound(t *testing.T) {
	s := newScene(t)
	id, _ := s.world.Stores.Conversations.Insert(core.Conversation{DomainId: "example.org", Kind: "direct"})
	s.world.Stores.Participants.Insert(core.Participant{DomainId: "example.org", ConversationId: "cnv_" + id, Kind: "member", MemberId: "ana"})
	sendRequest := request("c1", "hi")
	sendRequest.ConversationId = "cnv_" + id
	_, err := s.send(sendRequest)
	wantNotFound(t, err)
}

func TestARetryReturnsTheFirstMessageAndSendsNothingTwice(t *testing.T) {
	s := newScene(t)
	sendRequest := request("c1", "Hello", to("carol@partner.org", "email", "external"))
	sendRequest.Email.Subject = "Hello"
	first := s.mustSend(t, sendRequest)
	second := s.mustSend(t, sendRequest)
	if first.Message.Message.Id != second.Message.Message.Id || len(s.messages()) != 1 || len(s.conversations()) != 1 {
		t.Fatalf("a retry created a second message: %d messages", len(s.messages()))
	}
	message := s.stored(t, second)
	created(t, message)
	if len(emailRequests(t, message)) != 1 || rowsText(second.Deliveries) != rowsText(first.Deliveries) {
		t.Fatal("a retry must not stage a second email")
	}
}

func TestARetryStagesEventsThatWereNotStaged(t *testing.T) {
	s := newScene(t)
	conversation := core.Conversation{DomainId: "example.com", Kind: "direct"}
	id, _ := s.world.Stores.Conversations.Insert(conversation)
	for _, member := range []string{"ana", "bo"} {
		s.world.Stores.Participants.Insert(core.Participant{DomainId: "example.com", ConversationId: "cnv_" + id, Kind: "member", MemberId: member})
	}
	rows := []core.DeliveryRecord{{Recipient: "bo@example.com", MemberId: "bo", Channel: "chat", Role: "to", Status: "delivered"}}
	half := core.Message{DomainId: "example.com", ConversationId: "cnv_" + id, Month: "202610", Sequence: 1, SendKey: (Hashes{}).sendKey("ana", "c1"), BlocksJson: "[]", Deliveries: core.DeliveryCodec{}.Encode(rows)}
	messageId, _ := s.world.Stores.Messages.Insert(half)
	outcome := s.mustSend(t, request("c1", "ignored", to("bo@example.com", "chat", "member")))
	if outcome.Message.Message.Id != messageId || len(s.messages()) != 1 {
		t.Fatalf("retry made a new message: %+v", outcome.Message.Message)
	}
	message := s.stored(t, outcome)
	if !message.EventsStaged || !slices.Equal(created(t, message).MemberIds, []string{"ana", "bo"}) {
		t.Fatalf("events not staged: %+v", message)
	}
	again := s.mustSend(t, request("c1", "ignored", to("bo@example.com", "chat", "member")))
	created(t, s.stored(t, again))
}

func TestBccIsNeverAParticipantNorAVisibleRecipient(t *testing.T) {
	s := newScene(t)
	sendRequest := request("c1", "fyi", to("carol@partner.org", "email", "external"), choice("dan@partner.org", "bcc", "email", "external"), choice("cy@example.com", "cc", "email", "member"))
	sendRequest.Email.Subject = "Quote"
	outcome := s.mustSend(t, sendRequest)
	message := s.stored(t, outcome)
	for _, participant := range s.participants() {
		if participant.Address == "dan@partner.org" {
			t.Fatal("Bcc became a participant")
		}
	}
	if slices.Contains(message.Email.To, "dan@partner.org") || slices.Contains(message.Email.Cc, "dan@partner.org") || !slices.Equal(message.Email.Cc, []string{"cy@example.com"}) {
		t.Fatalf("headers %+v", message.Email)
	}
	if !strings.Contains(rowsText(outcome.Deliveries), "dan@partner.org/email/bcc/queued;") {
		t.Fatalf("the author must see the Bcc row: %s", rowsText(outcome.Deliveries))
	}
	roles := map[string]string{}
	for _, request := range emailRequests(t, message) {
		roles[request.Rcpt] = request.Role
		if slices.Contains(request.To, "dan@partner.org") || slices.Contains(request.Cc, "dan@partner.org") {
			t.Fatalf("Bcc leaked into %+v", request)
		}
	}
	if roles["dan@partner.org"] != "bcc" || roles["cy@example.com"] != "cc" || len(roles) != 3 {
		t.Fatalf("roles %v", roles)
	}
}

func TestAddingAnOutsidePersonToADirectConversation(t *testing.T) {
	s := newScene(t)
	first := s.mustSend(t, request("c1", "hi", to("bo@example.com", "chat", "member")))
	add := request("c2", "adding Carol", to("carol@partner.org", "email", "external"))
	add.ConversationId = first.Message.Message.ConversationId
	add.Email.Subject = "Welcome"
	outcome := s.mustSend(t, add)
	if got := rowsText(outcome.Deliveries); got != "carol@partner.org/email/to/queued;bo@example.com/chat/to/delivered;" {
		t.Fatalf("rows %s", got)
	}
	conversation := s.conversations()[0]
	if !conversation.HasExternal || conversation.ParticipantKey != "" || conversation.MessageCount != 2 || len(s.participants()) != 3 {
		t.Fatalf("conversation %+v participants %d", conversation, len(s.participants()))
	}
	message := s.stored(t, outcome)
	if !slices.Equal(message.Email.Cc, []string{"bo@example.com"}) {
		t.Fatalf("cc %v", message.Email.Cc)
	}
	again := request("c3", "follow up")
	again.ConversationId = first.Message.Message.ConversationId
	follow := s.mustSend(t, again)
	followed := s.stored(t, follow)
	if followed.Email.Subject != "Re: Welcome" || followed.Email.InReplyTo != message.Email.MessageIdHeader || rowsText(follow.Deliveries) != "bo@example.com/chat/to/delivered;carol@partner.org/email/to/queued;" {
		t.Fatalf("follow up %+v rows %s", followed.Email, rowsText(follow.Deliveries))
	}
	fresh := s.mustSend(t, request("c4", "just Bo", to("bo@example.com", "chat", "member")))
	if fresh.Message.Message.ConversationId == first.Message.Message.ConversationId {
		t.Fatal("a conversation with Carol must not be reused as the DM with Bo")
	}
}

func TestTheReplySubjectIsNotDoubled(t *testing.T) {
	s := newScene(t)
	first := request("c1", "a", to("carol@partner.org", "email", "external"))
	first.Email.Subject = "RE: Offer"
	outcome := s.mustSend(t, first)
	reply := request("c2", "b")
	reply.ConversationId = outcome.Message.Message.ConversationId
	replied := s.stored(t, s.mustSend(t, reply))
	if replied.Email.Subject != "RE: Offer" {
		t.Fatalf("subject %q", replied.Email.Subject)
	}
}

func TestAnEmailReplyWithoutAnySubjectIsInvalid(t *testing.T) {
	s := newScene(t)
	first := s.mustSend(t, request("c1", "hi", to("bo@example.com", "chat", "member")))
	add := request("c2", "adding Carol", to("carol@partner.org", "email", "external"))
	add.ConversationId = first.Message.Message.ConversationId
	_, err := s.send(add)
	wantInvalid(t, err, "subject")
}

func TestRecipientsCannotBeAddedToAChannelConversation(t *testing.T) {
	s := newScene(t)
	s.channel(t, false)
	sendRequest := request("c1", "hi", to("cy@example.com", "chat", "member"))
	sendRequest.ConversationId = "cnv_chan1"
	_, err := s.send(sendRequest)
	wantInvalid(t, err, "recipients")
	sendRequest.ConversationId = "cnv_000000000099"
	sendRequest.Recipients = []core.RecipientChoice{to("releases@example.com", "chat", "channel")}
	_, err = s.send(sendRequest)
	wantInvalid(t, err, "recipients")
}

func TestTheFromAddressMustBeTheCallersOwn(t *testing.T) {
	s := newScene(t)
	s.world.Directory.Entries["sales@example.com"] = core.AddressEntry{Address: "sales@example.com", DomainId: "example.com", Kind: "alias", TargetId: "ana"}
	s.world.Directory.Entries["boss@example.com"] = core.AddressEntry{Address: "boss@example.com", DomainId: "example.com", Kind: "alias", TargetId: "bo"}
	cases := map[string]bool{"Sales@Example.com": true, "ana@example.com": true, "boss@example.com": false, "bo@example.com": false, "x@nowhere.org": false}
	index := 0
	for address, allowed := range cases {
		index++
		sendRequest := request(fmt.Sprintf("c%d", index), "hi", to("carol@partner.org", "email", "external"))
		sendRequest.Email = core.EmailOptions{Subject: "S", FromAddress: address}
		outcome, err := s.send(sendRequest)
		if allowed && (err != nil || outcome.Message.Message.Email.From != strings.ToLower(address)) {
			t.Fatalf("%s should be allowed: %v %+v", address, err, outcome.Message.Message.Email)
		}
		if !allowed {
			wantInvalid(t, err, "fromAddress")
		}
	}
}

func TestMentionsNameReadersOnly(t *testing.T) {
	s := newScene(t)
	sendRequest := request("c1", "Hi @Bo, @bo. and @ana and @cy and @nobody, mail me at ana@example.com", to("bo@example.com", "chat", "member"))
	outcome := s.mustSend(t, sendRequest)
	message := s.stored(t, outcome)
	if !slices.Equal(message.MentionedMemberIds, []string{"bo"}) || !slices.Equal(created(t, message).MentionedMemberIds, []string{"bo"}) {
		t.Fatalf("mentions %v", message.MentionedMemberIds)
	}
	mentions := s.mentions()
	if len(mentions) != 1 || mentions[0].MemberId != "bo" || mentions[0].Sequence != 1 || mentions[0].MessageId != wireOf(message) {
		t.Fatalf("mention rows %+v", mentions)
	}
}

func TestMentionRowsStopAtTheWriteBudget(t *testing.T) {
	s := newScene(t)
	recipients := []core.RecipientChoice{to("bo@example.com", "chat", "member")}
	for index := 1; index <= 19; index++ {
		recipients = append(recipients, to(fmt.Sprintf("x%d@partner.org", index), "email", "external"))
	}
	sendRequest := request("c1", "hi @bo", recipients...)
	sendRequest.Email.Subject = "Many"
	outcome := s.mustSend(t, sendRequest)
	if !slices.Equal(outcome.Message.Message.MentionedMemberIds, []string{"bo"}) || len(s.mentions()) != 0 {
		t.Fatalf("mentions %v rows %d", outcome.Message.Message.MentionedMemberIds, len(s.mentions()))
	}
}

func TestASendOverTheWriteBudgetIsRefused(t *testing.T) {
	s := newScene(t)
	first := s.mustSend(t, request("c1", "root", to("bo@example.com", "chat", "member")))
	recipients := []core.RecipientChoice{}
	for index := 1; index <= 20; index++ {
		recipients = append(recipients, to(fmt.Sprintf("x%d@partner.org", index), "email", "external"))
	}
	reply := request("c2", "many", recipients...)
	reply.ConversationId = first.Message.Message.ConversationId
	reply.ThreadRootId = wireOf(first.Message.Message)
	reply.Email.Subject = "Many"
	_, err := s.send(reply)
	wantInvalid(t, err, "recipients")
	if len(s.messages()) != 1 || len(s.participants()) != 2 {
		t.Fatal("nothing may be written")
	}
}

func TestAMessageWithOnlyAnAttachmentIsAllowed(t *testing.T) {
	s := newScene(t)
	sendRequest := request("c1", "  ", to("bo@example.com", "chat", "member"))
	sendRequest.AttachmentIds = []string{"att_1"}
	outcome := s.mustSend(t, sendRequest)
	if !slices.Equal(outcome.Message.Message.AttachmentIds, []string{"att_1"}) || !slices.Equal(created(t, outcome.Message.Message).AttachmentIds, []string{"att_1"}) {
		t.Fatalf("attachments %v", outcome.Message.Message.AttachmentIds)
	}
}

func storedUses(s *scene) []core.AttachmentUse {
	return s.world.Stores.AttachmentUses.(*core.MemoryAttachmentUses).All()
}

func TestEachAttachmentOfANewMessageGetsAUseRow(t *testing.T) {
	s := newScene(t)
	sendRequest := request("c1", "files", to("bo@example.com", "chat", "member"))
	sendRequest.AttachmentIds = []string{"att_1", "att_2"}
	outcome := s.mustSend(t, sendRequest)
	message := outcome.Message.Message
	uses := storedUses(s)
	if len(uses) != 2 || uses[0].AttachmentId != "att_1" || uses[1].AttachmentId != "att_2" {
		t.Fatalf("uses %+v", uses)
	}
	if uses[0].DomainId != "example.com" || uses[0].MessageId != wireOf(message) || uses[0].ConversationId != message.ConversationId {
		t.Fatalf("use %+v", uses[0])
	}
	s.mustSend(t, sendRequest)
	if len(storedUses(s)) != 2 {
		t.Fatal("a finished retry must not write the uses again")
	}
}

func TestARetryWritesTheUsesOfAMessageThatWasNotFinished(t *testing.T) {
	s := newScene(t)
	id, _ := s.world.Stores.Conversations.Insert(core.Conversation{DomainId: "example.com", Kind: "direct"})
	half := core.Message{DomainId: "example.com", ConversationId: "cnv_" + id, Month: "202611", SendKey: (Hashes{}).sendKey("ana", "c1"), BlocksJson: "[]", AttachmentIds: []string{"att_1"}}
	s.world.Stores.Messages.Insert(half)
	s.mustSend(t, request("c1", "ignored", to("bo@example.com", "chat", "member")))
	uses := storedUses(s)
	if len(uses) != 1 || uses[0].AttachmentId != "att_1" {
		t.Fatalf("uses %+v", uses)
	}
	if names := created(t, s.messages()[0]).FileNames; len(names) != 0 {
		t.Fatalf("an old message has no file names, got %v", names)
	}
}

func TestAttachmentsCountInTheWriteBudget(t *testing.T) {
	s := newScene(t)
	recipients := []core.RecipientChoice{}
	for index := 1; index <= 12; index++ {
		recipients = append(recipients, to(fmt.Sprintf("x%d@partner.org", index), "email", "external"))
	}
	sendRequest := request("c1", "many", recipients...)
	sendRequest.Email.Subject = "Many"
	for index := 0; index < 10; index++ {
		sendRequest.AttachmentIds = append(sendRequest.AttachmentIds, fmt.Sprintf("att_%d", index))
	}
	_, err := s.send(sendRequest)
	wantInvalid(t, err, "recipients")
	if len(s.messages()) != 0 || len(storedUses(s)) != 0 {
		t.Fatal("nothing may be written")
	}
}

func TestAReplyCanAlsoGoToTheConversation(t *testing.T) {
	s := newScene(t)
	first := s.mustSend(t, request("c1", "root", to("bo@example.com", "chat", "member")))
	reply := request("c2", "reply")
	reply.ConversationId = first.Message.Message.ConversationId
	reply.ThreadRootId = wireOf(first.Message.Message)
	reply.AlsoToConversation = true
	outcome := s.mustSend(t, reply)
	if !outcome.Message.Message.AlsoToConversation || !s.stored(t, outcome).AlsoToConversation || s.stored(t, first).AlsoToConversation {
		t.Fatalf("the flag was not stored: %+v", outcome.Message.Message)
	}
}
