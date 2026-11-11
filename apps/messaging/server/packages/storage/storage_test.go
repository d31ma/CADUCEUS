package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

type memorySource struct {
	collections map[string]map[string]map[string]any
	order       map[string][]string
	failure     error
	next        int
}

func newMemorySource() *memorySource {
	return &memorySource{collections: map[string]map[string]map[string]any{}, order: map[string][]string{}}
}

func (source *memorySource) CollectionExists(name string) (bool, error) {
	_, exists := source.collections[name]
	return exists, source.failure
}

func (source *memorySource) CreateCollection(name string) error {
	source.collections[name] = map[string]map[string]any{}
	return source.failure
}

func (source *memorySource) Insert(collection string, document any) (string, error) {
	if source.failure != nil {
		return "", source.failure
	}
	source.next++
	id := fmt.Sprintf("%014d", source.next)
	source.collections[collection][id] = source.fields(document)
	source.order[collection] = append(source.order[collection], id)
	return id, nil
}

func (source *memorySource) FindById(collection string, id string, target any) (bool, error) {
	fields, found := source.collections[collection][id]
	if !found || source.failure != nil {
		return false, source.failure
	}
	encoded, _ := json.Marshal(fields)
	return true, json.Unmarshal(encoded, target)
}

func (source *memorySource) Patch(collection string, id string, fields any, target any) (bool, error) {
	existing, found := source.collections[collection][id]
	if !found {
		return false, source.failure
	}
	maps.Copy(existing, source.fields(fields))
	return true, source.failure
}

func (source *memorySource) DeleteById(collection string, id string) (bool, error) {
	_, found := source.collections[collection][id]
	delete(source.collections[collection], id)
	return found, source.failure
}

func (source *memorySource) Scroll(collection string, where map[string]any, cursor string, size int) (Window, error) {
	if source.failure != nil {
		return Window{}, source.failure
	}
	start, _ := strconv.Atoi(cursor)
	rows := []map[string]any{}
	ids := source.order[collection]
	index := start
	for ; index < len(ids) && len(rows) < size; index++ {
		fields, found := source.collections[collection][ids[index]]
		if found && source.matches(fields, where) {
			rows = append(rows, source.withId(fields, ids[index]))
		}
	}
	content, _ := json.Marshal(rows)
	return Window{Content: content, NextCursor: source.cursorAfter(index, len(ids))}, nil
}

func (source *memorySource) cursorAfter(index int, total int) string {
	if index >= total {
		return ""
	}
	return strconv.Itoa(index)
}

func (source *memorySource) matches(fields map[string]any, where map[string]any) bool {
	for name, value := range where {
		if fmt.Sprint(fields[name]) != fmt.Sprint(value) {
			return false
		}
	}
	return true
}

func (source *memorySource) withId(fields map[string]any, id string) map[string]any {
	copied := map[string]any{"id": id}
	maps.Copy(copied, fields)
	return copied
}

func (source *memorySource) fields(document any) map[string]any {
	encoded, _ := json.Marshal(document)
	fields := map[string]any{}
	json.Unmarshal(encoded, &fields)
	return fields
}

type countingPusher struct {
	pushed int
}

func (pusher *countingPusher) Push(subscriber outbox.Subscriber, envelope events.Envelope) error {
	pusher.pushed++
	return nil
}

func newStorage(source *memorySource) Storage {
	return Storage{Source: source, Scroll: source.Scroll, SettingsFailure: errors.New("no outbox settings")}
}

func TestDocumentsReadEveryPage(t *testing.T) {
	source := newMemorySource()
	documents := Documents{Source: source, Scroll: source.Scroll}
	if err := documents.Ensure("rows"); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 150; index++ {
		documents.Insert("rows", map[string]any{"index": index})
	}
	rows := []map[string]any{}
	if err := documents.All("rows", map[string]any{}, &rows); err != nil || len(rows) != 150 {
		t.Fatalf("want 150 rows, got %d (%v)", len(rows), err)
	}
}

func TestDocumentsFindReplaceAndDelete(t *testing.T) {
	source := newMemorySource()
	documents := Documents{Source: source, Scroll: source.Scroll}
	documents.Ensure("rows")
	id, _ := documents.Insert("rows", map[string]any{"id": "ignored", "name": "a"})
	documents.Replace("rows", id, map[string]any{"name": "b"})
	found := map[string]any{}
	exists, err := documents.Find("rows", id, &found)
	if err != nil || !exists || found["name"] != "b" || found["id"] != id {
		t.Fatalf("want the replaced row with its id, got %v %v %v", found, exists, err)
	}
	documents.Delete("rows", id)
	exists, _ = documents.Find("rows", id, &found)
	if exists {
		t.Fatal("the row must be gone")
	}
}

func TestDocumentFailuresAreUnavailable(t *testing.T) {
	source := newMemorySource()
	source.failure = errors.New("disk full")
	documents := Documents{Source: source, Scroll: source.Scroll}
	unavailable := &core.Unavailable{}
	_, existsError := documents.Exists("rows")
	_, insertError := documents.Insert("rows", map[string]any{})
	if !errors.As(existsError, &unavailable) || !errors.As(insertError, &unavailable) || !errors.As(documents.Ensure("rows"), &unavailable) {
		t.Fatalf("want Unavailable, got %v and %v", existsError, insertError)
	}
	if !errors.As(documents.All("rows", nil, &[]any{}), &unavailable) {
		t.Fatal("want Unavailable from All")
	}
}

func TestConversationsStayInTheirDomain(t *testing.T) {
	stores := newStorage(newMemorySource()).Stores()
	_, found, err := stores.Conversations.Find("a.test", "missing")
	if err != nil || found {
		t.Fatal("a missing collection finds nothing")
	}
	id, _ := stores.Conversations.Insert(core.Conversation{DomainId: "a.test", Key: "k1", ChannelId: "c1", ParticipantKey: "p1"})
	byKey, found, _ := stores.Conversations.Find("a.test", "k1")
	if !found || byKey.Id != id {
		t.Fatal("found by key")
	}
	byKey.MessageCount = 2
	stores.Conversations.Save(byKey)
	byChannel, found, _ := stores.Conversations.ByChannel("a.test", "c1")
	if !found || byChannel.MessageCount != 2 {
		t.Fatal("found by channel after save")
	}
	if _, found, _ := stores.Conversations.ByParticipantKey("a.test", "p1"); !found {
		t.Fatal("found by participant key")
	}
	if _, found, _ := stores.Conversations.Find("b.test", id); found {
		t.Fatal("another domain finds nothing")
	}
}

func TestParticipantsAreListedSavedAndDeleted(t *testing.T) {
	stores := newStorage(newMemorySource()).Stores()
	id, _ := stores.Participants.Insert(core.Participant{DomainId: "a.test", ConversationId: "cnv_1", MemberId: "ana"})
	listed, _ := stores.Participants.OfConversation("a.test", "cnv_1")
	listed[0].ReadSequence = 3
	stores.Participants.Save(listed[0])
	ofMember, _ := stores.Participants.OfMember("a.test", "ana")
	if len(ofMember) != 1 || ofMember[0].ReadSequence != 3 {
		t.Fatalf("want the saved participant, got %v", ofMember)
	}
	stores.Participants.Delete("b.test", id)
	stores.Participants.Delete("a.test", id)
	if left, _ := stores.Participants.OfMember("a.test", "ana"); len(left) != 0 {
		t.Fatal("the participant must be deleted")
	}
}

func TestMessagesReactionsAndLookups(t *testing.T) {
	stores := newStorage(newMemorySource()).Stores()
	id, _ := stores.Messages.Insert(core.Message{DomainId: "a.test", Month: "202611", ConversationId: "cnv_1", SendKey: "s1"})
	message, found, _ := stores.Messages.Find("a.test", "202611", id)
	message.Body = "hi"
	stores.Messages.Save(message)
	bySendKey, _, _ := stores.Messages.BySendKey("a.test", "202611", "s1")
	listed, _ := stores.Messages.OfConversation("a.test", "202611", "cnv_1")
	if !found || bySendKey.Body != "hi" || len(listed) != 1 {
		t.Fatal("messages are found, saved and listed")
	}
	stores.Reactions.Insert(core.Reaction{DomainId: "a.test", Key: "r1", ConversationId: "cnv_1", Active: true})
	reaction, _, _ := stores.Reactions.ByKey("a.test", "r1")
	reaction.Active = false
	stores.Reactions.Save(reaction)
	reactions, _ := stores.Reactions.OfConversation("a.test", "cnv_1")
	stores.ThreadKeys.Insert(core.ThreadKey{DomainId: "a.test", Key: "k1", MessageId: "msg_1"})
	threadKey, _, _ := stores.ThreadKeys.ByKey("a.test", "k1")
	stores.Mentions.Insert(core.Mention{DomainId: "a.test", MemberId: "ana"})
	mentions, _ := stores.Mentions.OfMember("a.test", "ana")
	stores.AttachmentUses.Insert(core.AttachmentUse{DomainId: "a.test", AttachmentId: "att_1"})
	uses, _ := stores.AttachmentUses.ByAttachment("a.test", "att_1")
	if len(reactions) != 1 || reactions[0].Active || threadKey.MessageId != "msg_1" || len(mentions) != 1 || len(uses) != 1 {
		t.Fatal("reactions, thread keys, mentions and attachment uses are stored")
	}
}

func TestMembersAreSavedOnce(t *testing.T) {
	stores := newStorage(newMemorySource()).Stores()
	stores.Members.Save(core.Member{DomainId: "a.test", MemberId: "ana", LocalPart: "ana", DisplayName: "Ana"})
	stores.Members.Save(core.Member{DomainId: "a.test", MemberId: "ana", LocalPart: "ana", DisplayName: "Ana B"})
	member, found, _ := stores.Members.ByLocalPart("a.test", "ana")
	again, _, _ := stores.Members.ByMemberId("a.test", "ana")
	if !found || member.DisplayName != "Ana B" || again.Id != member.Id {
		t.Fatalf("want one updated member, got %v", member)
	}
}

func TestSeenEventsArePerDomain(t *testing.T) {
	seen := newStorage(newMemorySource()).Seen()
	seen.Remember("a.test", "evt_1")
	here, _ := seen.Seen("a.test", "evt_1")
	there, _ := seen.Seen("b.test", "evt_1")
	if !here || there {
		t.Fatal("seen only in its own domain")
	}
}

func TestTrackedWritesRegisterTheirCollectionAndDrain(t *testing.T) {
	source := newMemorySource()
	storage := newStorage(source)
	drained := []outbox.Ref{}
	tenant := Tenant{Documents: storage.documents(), Registry: storage.Registry(), Drain: func(ref outbox.Ref) { drained = append(drained, ref) }}
	id, _ := tenant.InsertTracked("messages-x", map[string]any{"domainId": "a.test"}, 1)
	tenant.ReplaceTracked("messages-x", id, map[string]any{"domainId": "a.test"}, 0)
	names, _ := storage.Registry().All()
	if len(drained) != 1 || drained[0].Id != id || len(names) != 1 || names[0] != "messages-x" {
		t.Fatalf("want one drain and one registered collection, got %v and %v", drained, names)
	}
	if storage.Registry().Check("messages-x") != nil {
		t.Fatal("a registered collection passes the check")
	}
}

func TestTheOutboxStoreListsPendingAndDeadRows(t *testing.T) {
	source := newMemorySource()
	storage := newStorage(source)
	store := storage.Outbox().Store
	tenant := storage.tenant()
	pendingId, _ := tenant.InsertTracked("messages-x", map[string]any{"domainId": "a.test", "outboxPending": 1}, 0)
	tenant.InsertTracked("messages-x", map[string]any{"domainId": "a.test", "outboxDead": 1}, 0)
	pending, _ := store.Pending(10)
	dead, _ := store.Dead()
	fields, err := store.Load(outbox.Ref{Collection: "messages-x", Id: pendingId})
	if len(pending) != 1 || pending[0].Ref.Id != pendingId || len(dead) != 1 || err != nil || fields.OutboxPending != 1 {
		t.Fatalf("want one pending and one dead row, got %v and %v", pending, dead)
	}
	if err := store.Save(outbox.Ref{Collection: "messages-x", Id: pendingId}, outbox.Fields{}); err != nil {
		t.Fatal(err)
	}
	if empty, _ := store.Pending(0); len(empty) != 0 {
		t.Fatal("a limit of zero lists nothing")
	}
}

func TestUnknownOutboxRowsAreNotFound(t *testing.T) {
	storage := newStorage(newMemorySource())
	store := storage.Outbox().Store
	missing := &core.NotFound{}
	_, unknownCollection := store.Load(outbox.Ref{Collection: "nothing", Id: "1"})
	storage.tenant().InsertTracked("messages-x", map[string]any{}, 0)
	_, unknownRow := store.Load(outbox.Ref{Collection: "messages-x", Id: "404"})
	saveError := store.Save(outbox.Ref{Collection: "nothing", Id: "1"}, outbox.Fields{})
	retryError := storage.Outbox().Retry(outbox.Ref{Collection: "messages-x", Id: "404"}, "evt_1", "search")
	if !errors.As(unknownCollection, &missing) || !errors.As(unknownRow, &missing) || !errors.As(saveError, &missing) || !errors.As(retryError, &missing) {
		t.Fatalf("want NotFound, got %v, %v, %v, %v", unknownCollection, unknownRow, saveError, retryError)
	}
}

func TestMissingSettingsStopTheRelay(t *testing.T) {
	storage := newStorage(newMemorySource())
	_, err := storage.Outbox().RunOnce(5)
	storage.Outbox().Drain(outbox.Ref{Collection: "messages-x", Id: "1"})
	if err == nil || err.Error() != "no outbox settings" {
		t.Fatalf("want the settings failure, got %v", err)
	}
}

func TestTheRelayRunsWithSettings(t *testing.T) {
	source := newMemorySource()
	storage := Storage{Source: source, Scroll: source.Scroll, Pusher: &countingPusher{}}
	storage.tenant().InsertTracked("messages-x", map[string]any{"domainId": "a.test"}, 1)
	report, err := storage.Outbox().RunOnce(5)
	dead, _ := storage.Outbox().Dead()
	if err != nil || report.Documents != 0 || len(dead) != 0 {
		t.Fatalf("want an empty run, got %v %v", report, err)
	}
}
