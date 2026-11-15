package syncing_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/fakes"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/syncing"
)

type world struct {
	t      *testing.T
	fakes  *fakes.FakeWorld
	pushed []string
	log    syncing.ChangeLog
	events int
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{t: t, fakes: fakes.NewFakeWorld()}
	w.log = syncing.NewChangeLog(outbox.PushFunc(func(subscriber outbox.Subscriber, envelope events.Envelope) error {
		w.pushed = append(w.pushed, subscriber.Name+"/"+envelope.EventID)
		return nil
	}), w.fakes.Stores, w.fakes.Clock.Now)
	return w
}

func (w *world) caller(memberId string) oidc.Caller {
	return oidc.Caller{DomainId: "example.com", MemberId: memberId}
}

func (w *world) conversation() core.Conversation {
	conversation := core.Conversation{DomainId: "example.com", Kind: "direct", CreatedMonth: "202611"}
	id, _ := w.fakes.Stores.Conversations.Insert(conversation)
	conversation.Id = id
	for _, memberId := range []string{"ana", "bo"} {
		w.fakes.Stores.Participants.Insert(core.Participant{DomainId: "example.com", ConversationId: cnvId(conversation), Kind: "member", MemberId: memberId})
	}
	return conversation
}

func (w *world) post(conversation core.Conversation, body string) core.Message {
	message := core.Message{DomainId: "example.com", ConversationId: cnvId(conversation), Month: "202611", AuthorKind: "member", AuthorMemberId: "ana", Body: body, Version: 1}
	id, _ := w.fakes.Stores.Messages.Insert(message)
	message.Id = id
	return message
}

func (w *world) envelope(eventType string, payload any) events.Envelope {
	w.events++
	encoded, _ := json.Marshal(payload)
	return events.Envelope{EventID: fmt.Sprintf("event-%d", w.events), Type: eventType, DomainId: "example.com", OccurredAt: w.fakes.Clock.Now().Format(time.RFC3339Nano), Payload: encoded}
}

func (w *world) created(message core.Message, rootId string) events.Envelope {
	return w.envelope(events.Types{}.MessageCreated(), events.MessageCreated{MessageId: msgId(message), ConversationId: message.ConversationId, ThreadRootId: rootId})
}

func (w *world) edited(message core.Message) events.Envelope {
	return w.envelope(events.Types{}.MessageEdited(), events.MessageEdited{MessageId: msgId(message), ConversationId: message.ConversationId})
}

func (w *world) record(envelope events.Envelope) {
	w.t.Helper()
	w.fakes.Clock.Advance(time.Second)
	if err := w.log.Record(envelope); err != nil {
		w.t.Fatal(err)
	}
}

func (w *world) rows() []core.Change {
	return w.fakes.Stores.Changes.(*core.MemoryChanges).All()
}

func (w *world) head(conversation core.Conversation) string {
	head, _, _ := w.fakes.Stores.ChangeHeads.Find("example.com", cnvId(conversation))
	return head.LastChangeId
}

func (w *world) sync(memberId string, cursors ...core.SyncCursor) []core.ConversationSync {
	w.t.Helper()
	answers, err := syncing.NewSync(w.fakes.Stores, w.fakes.Ports).Run(w.caller(memberId), cursors)
	if err != nil {
		w.t.Fatal(err)
	}
	return answers
}

func (w *world) syncOne(conversation core.Conversation, after string) core.ConversationSync {
	w.t.Helper()
	return w.sync("bo", core.SyncCursor{ConversationId: cnvId(conversation), After: after})[0]
}

func cnvId(conversation core.Conversation) string {
	return core.ConversationIds{}.Wire(conversation)
}

func msgId(message core.Message) string {
	return core.MessageIds{}.Wire(message.Month, message.Id)
}

func bodies(answer core.ConversationSync) []string {
	found := []string{}
	for _, view := range answer.Messages {
		found = append(found, view.Message.Body)
	}
	return found
}

type failingChanges struct {
	core.ChangeStore
}

func (failingChanges) OfEvent(domainId string, month string, eventId string) ([]core.Change, error) {
	return nil, errors.New("changes are down")
}

func (failingChanges) OfConversation(domainId string, month string, conversationId string) ([]core.Change, error) {
	return nil, errors.New("changes are down")
}

type failingAppends struct {
	core.ChangeStore
}

func (failingAppends) Append(change core.Change) (string, error) {
	return "", errors.New("changes are down")
}

type failingHeads struct {
	core.ChangeHeadStore
	saves bool
}

func (heads failingHeads) Find(domainId string, conversationId string) (core.ChangeHead, bool, error) {
	if heads.saves {
		return heads.ChangeHeadStore.Find(domainId, conversationId)
	}
	return core.ChangeHead{}, false, errors.New("heads are down")
}

func (failingHeads) Insert(head core.ChangeHead) (string, error) {
	return "", errors.New("heads are down")
}

type failingMessages struct {
	core.MessageStore
}

func (failingMessages) Find(domainId string, month string, id string) (core.Message, bool, error) {
	return core.Message{}, false, errors.New("messages are down")
}
