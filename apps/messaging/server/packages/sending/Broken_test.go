package sending

import (
	"errors"
	"io"
	"testing"

	"mesail.dev/packages/core"
)

type breaker struct {
	operation string
	skip      int
	calls     int
}

func (b *breaker) fails(operation string) bool {
	if operation != b.operation {
		return false
	}
	b.calls++
	return b.calls > b.skip
}

func boom() error {
	return errors.New("boom")
}

type brokenConversations struct {
	core.ConversationStore
	b *breaker
}

func (store brokenConversations) Insert(row core.Conversation) (string, error) {
	if store.b.fails("conversations.Insert") {
		return "", boom()
	}
	return store.ConversationStore.Insert(row)
}

func (store brokenConversations) Save(row core.Conversation) error {
	if store.b.fails("conversations.Save") {
		return boom()
	}
	return store.ConversationStore.Save(row)
}

func (store brokenConversations) Find(domainId string, storedPart string) (core.Conversation, bool, error) {
	if store.b.fails("conversations.Find") {
		return core.Conversation{}, false, boom()
	}
	return store.ConversationStore.Find(domainId, storedPart)
}

func (store brokenConversations) ByChannel(domainId string, channelId string) (core.Conversation, bool, error) {
	if store.b.fails("conversations.ByChannel") {
		return core.Conversation{}, false, boom()
	}
	return store.ConversationStore.ByChannel(domainId, channelId)
}

func (store brokenConversations) ByParticipantKey(domainId string, key string) (core.Conversation, bool, error) {
	if store.b.fails("conversations.ByParticipantKey") {
		return core.Conversation{}, false, boom()
	}
	return store.ConversationStore.ByParticipantKey(domainId, key)
}

type brokenParticipants struct {
	core.ParticipantStore
	b *breaker
}

func (store brokenParticipants) Insert(row core.Participant) (string, error) {
	if store.b.fails("participants.Insert") {
		return "", boom()
	}
	return store.ParticipantStore.Insert(row)
}

func (store brokenParticipants) Save(row core.Participant) error {
	if store.b.fails("participants.Save") {
		return boom()
	}
	return store.ParticipantStore.Save(row)
}

func (store brokenParticipants) OfConversation(domainId string, conversationId string) ([]core.Participant, error) {
	if store.b.fails("participants.OfConversation") {
		return nil, boom()
	}
	return store.ParticipantStore.OfConversation(domainId, conversationId)
}

func (store brokenParticipants) OfMember(domainId string, memberId string) ([]core.Participant, error) {
	if store.b.fails("participants.OfMember") {
		return nil, boom()
	}
	return store.ParticipantStore.OfMember(domainId, memberId)
}

type brokenMessages struct {
	core.MessageStore
	b *breaker
}

func (store brokenMessages) Insert(row core.Message) (string, error) {
	if store.b.fails("messages.Insert") {
		return "", boom()
	}
	return store.MessageStore.Insert(row)
}

func (store brokenMessages) Save(row core.Message) error {
	if store.b.fails("messages.Save") {
		return boom()
	}
	return store.MessageStore.Save(row)
}

func (store brokenMessages) Find(domainId string, month string, id string) (core.Message, bool, error) {
	if store.b.fails("messages.Find") {
		return core.Message{}, false, boom()
	}
	return store.MessageStore.Find(domainId, month, id)
}

func (store brokenMessages) BySendKey(domainId string, month string, key string) (core.Message, bool, error) {
	if store.b.fails("messages.BySendKey") {
		return core.Message{}, false, boom()
	}
	return store.MessageStore.BySendKey(domainId, month, key)
}

func (store brokenMessages) OfConversation(domainId string, month string, conversationId string) ([]core.Message, error) {
	if store.b.fails("messages.OfConversation") {
		return nil, boom()
	}
	return store.MessageStore.OfConversation(domainId, month, conversationId)
}

type brokenThreadKeys struct {
	core.ThreadKeyStore
	b *breaker
}

func (store brokenThreadKeys) Insert(row core.ThreadKey) (string, error) {
	if store.b.fails("threadKeys.Insert") {
		return "", boom()
	}
	return store.ThreadKeyStore.Insert(row)
}

type brokenMentions struct {
	core.MentionStore
	b *breaker
}

func (store brokenMentions) Insert(row core.Mention) (string, error) {
	if store.b.fails("mentions.Insert") {
		return "", boom()
	}
	return store.MentionStore.Insert(row)
}

type brokenAttachmentUses struct {
	core.AttachmentUseStore
	b *breaker
}

func (store brokenAttachmentUses) Insert(row core.AttachmentUse) (string, error) {
	if store.b.fails("attachmentUses.Insert") {
		return "", boom()
	}
	return store.AttachmentUseStore.Insert(row)
}

type brokenMembers struct {
	core.MemberStore
	b *breaker
}

func (store brokenMembers) ByLocalPart(domainId string, localPart string) (core.Member, bool, error) {
	if store.b.fails("members.ByLocalPart") {
		return core.Member{}, false, boom()
	}
	return store.MemberStore.ByLocalPart(domainId, localPart)
}

func brokenStores(stores core.Stores, b *breaker) core.Stores {
	return core.Stores{
		Conversations:  brokenConversations{stores.Conversations, b},
		Participants:   brokenParticipants{stores.Participants, b},
		Messages:       brokenMessages{stores.Messages, b},
		Reactions:      stores.Reactions,
		ThreadKeys:     brokenThreadKeys{stores.ThreadKeys, b},
		Mentions:       brokenMentions{stores.Mentions, b},
		Members:        brokenMembers{stores.Members, b},
		AttachmentUses: brokenAttachmentUses{stores.AttachmentUses, b},
	}
}

type silentDirectory struct {
	core.Directory
}

func (silentDirectory) Resolve(authorization string, addresses []string) ([]core.Resolved, error) {
	return []core.Resolved{}, nil
}

type addressFailure struct {
	core.Directory
}

func (addressFailure) Address(domainId string, address string) (core.AddressEntry, bool, error) {
	return core.AddressEntry{}, false, boom()
}

type limitedRandom struct {
	reads int
}

func (random *limitedRandom) Read(buffer []byte) (int, error) {
	if random.reads <= 0 {
		return 0, io.ErrUnexpectedEOF
	}
	random.reads--
	return len(buffer), nil
}

func portsWith(s *scene, directory core.Directory) core.Ports {
	ports := s.world.Ports
	ports.Directory = directory
	return ports
}

func emailTo(clientMessageId string) core.SendRequest {
	sendRequest := request(clientMessageId, "hi @bo", to("carol@partner.org", "email", "external"), to("bo@example.com", "chat", "member"))
	sendRequest.Email.Subject = "S"
	return sendRequest
}

func TestStoreFailuresAnswerUnavailable(t *testing.T) {
	type failure struct {
		operation string
		skip      int
		setup     func(s *scene) core.SendRequest
	}
	chat := func(s *scene) core.SendRequest {
		return request("c9", "hi @bo", to("bo@example.com", "chat", "member"))
	}
	email := func(s *scene) core.SendRequest {
		return emailTo("c9")
	}
	selfNote := func(s *scene) core.SendRequest {
		return request("c9", "note", to("ana@example.com", "chat", "member"))
	}
	existing := func(s *scene) core.SendRequest {
		first := s.mustSend(t, emailTo("c1"))
		reply := request("c9", "reply")
		reply.ConversationId = first.Message.Message.ConversationId
		reply.ThreadRootId = wireOf(first.Message.Message)
		return reply
	}
	reuse := func(s *scene) core.SendRequest {
		s.mustSend(t, request("c1", "first", to("bo@example.com", "chat", "member")))
		return chat(s)
	}
	channel := func(s *scene) core.SendRequest {
		s.channel(t, false)
		return request("c9", "hi", to("releases@example.com", "chat", "channel"))
	}
	unstaged := func(s *scene) core.SendRequest {
		id, _ := s.world.Stores.Conversations.Insert(core.Conversation{DomainId: "example.com", Kind: "direct"})
		s.world.Stores.Messages.Insert(core.Message{DomainId: "example.com", ConversationId: "cnv_" + id, Month: "202611", SendKey: (Hashes{}).sendKey("ana", "c9"), BlocksJson: "[]", AttachmentIds: []string{"att_1"}})
		return chat(s)
	}
	attached := func(s *scene) core.SendRequest {
		sendRequest := chat(s)
		sendRequest.AttachmentIds = []string{"att_1"}
		return sendRequest
	}
	failures := []failure{
		{"messages.BySendKey", 0, chat},
		{"conversations.ByParticipantKey", 0, chat},
		{"conversations.Insert", 0, chat},
		{"participants.Insert", 0, chat},
		{"participants.Insert", 0, selfNote},
		{"messages.Insert", 0, chat},
		{"messages.Save", 0, chat},
		{"mentions.Insert", 0, chat},
		{"members.ByLocalPart", 0, chat},
		{"threadKeys.Insert", 0, email},
		{"messages.OfConversation", 0, email},
		{"conversations.Save", 0, existing},
		{"participants.OfConversation", 0, existing},
		{"participants.Save", 0, existing},
		{"messages.Find", 0, existing},
		{"messages.Save", 1, existing},
		{"participants.OfConversation", 0, reuse},
		{"participants.OfMember", 0, reuse},
		{"conversations.ByChannel", 0, channel},
		{"participants.OfConversation", 0, channel},
		{"conversations.Find", 0, unstaged},
		{"participants.OfConversation", 0, unstaged},
		{"messages.Save", 0, unstaged},
		{"attachmentUses.Insert", 0, unstaged},
		{"attachmentUses.Insert", 0, attached},
	}
	for _, failure := range failures {
		s := newScene(t)
		sendRequest := failure.setup(s)
		b := &breaker{operation: failure.operation, skip: failure.skip}
		sender := NewSender(brokenStores(s.world.Stores, b), s.world.Ports)
		_, err := sender.Send(s.ana, "Bearer ana-token", sendRequest)
		if !errors.As(err, new(*core.Unavailable)) {
			t.Fatalf("%s: want Unavailable, got %#v", failure.operation, err)
		}
	}
}

func TestPortFailuresAnswerUnavailable(t *testing.T) {
	s := newScene(t)
	s.world.Directory.Failure = boom()
	_, err := s.send(request("c1", "hi", to("bo@example.com", "chat", "member")))
	wantUnavailable(t, err)

	s = newScene(t)
	s.world.Renderer.Failure = boom()
	_, err = s.send(request("c1", "hi", to("bo@example.com", "chat", "member")))
	wantUnavailable(t, err)

	s = newScene(t)
	s.sender = NewSender(s.world.Stores, portsWith(s, addressFailure{s.world.Directory}))
	sendRequest := emailTo("c1")
	sendRequest.Email.FromAddress = "sales@example.com"
	_, err = s.send(sendRequest)
	wantUnavailable(t, err)
}

func TestStagingFailuresStopTheSend(t *testing.T) {
	for _, reads := range []int{0, 1} {
		s := newScene(t)
		ports := s.world.Ports
		ports.Stager = core.NewStager(s.world.OutboxSettings(), s.world.Clock.Now, &limitedRandom{reads: reads})
		_, err := NewSender(s.world.Stores, ports).Send(s.ana, "Bearer ana-token", emailTo("c1"))
		if err == nil {
			t.Fatalf("reads %d: want a staging failure", reads)
		}
	}
}

func TestDamagedStoredMessagesAreReported(t *testing.T) {
	damaged := []core.Message{
		{ConversationId: "cnv_x", BlocksJson: "[]", Deliveries: []string{"not json"}},
		{ConversationId: "not a wire id", BlocksJson: "[]"},
		{ConversationId: "cnv_x", BlocksJson: "{broken", EventsStaged: true},
		{ConversationId: "cnv_x", BlocksJson: "[]", Deliveries: []string{"not json"}, EventsStaged: true},
	}
	for index, message := range damaged {
		s := newScene(t)
		message.DomainId = "example.com"
		message.Month = "202611"
		message.SendKey = (Hashes{}).sendKey("ana", "c1")
		s.world.Stores.Messages.Insert(message)
		_, err := s.send(request("c1", "hi", to("bo@example.com", "chat", "member")))
		if err == nil {
			t.Fatalf("case %d: want an error", index)
		}
	}
}
