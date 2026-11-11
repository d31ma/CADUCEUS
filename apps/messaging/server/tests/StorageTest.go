import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

// @Test(StorageTestSetup, {"partition":"tests"})
type StorageTest struct {
	behaviors *BehaviorContext
}

func (test *StorageTest) TestConversationsAreFoundByIdKeyChannelAndParticipants() {
	world := StorageTestSetup{}.World()
	store := world.Conversations()
	_, found, err := store.Find(world.Domain, "cnvmissing")
	Assert.IsTrue(err == nil && !found, "a missing collection finds nothing")
	id, err := store.Insert(core.Conversation{DomainId: world.Domain, Key: "chan1", Kind: "channel", ChannelId: "chan1", ParticipantKey: "pk1"})
	world.Must(err)
	byId, found, err := store.Find(world.Domain, id)
	Assert.IsTrue(err == nil && found, "found by id")
	Assert.Equals(id, byId.Id)
	byKey, found, _ := store.Find(world.Domain, "chan1")
	Assert.IsTrue(found, "found by key")
	Assert.Equals(id, byKey.Id)
	byKey.MessageCount = 3
	world.Must(store.Save(byKey))
	byChannel, found, _ := store.ByChannel(world.Domain, "chan1")
	Assert.IsTrue(found, "found by channel")
	Assert.Equals(3, byChannel.MessageCount)
	byParticipants, found, _ := store.ByParticipantKey(world.Domain, "pk1")
	Assert.IsTrue(found, "found by participant key")
	Assert.Equals(id, byParticipants.Id)
	_, found, err = store.Find("other-"+world.Domain, id)
	Assert.IsTrue(err == nil && !found, "another domain finds nothing")
}

func (test *StorageTest) TestParticipantsDrainTheirEventsAndCanBeDeleted() {
	world := StorageTestSetup{}.World()
	store := world.Participants()
	participant := core.Participant{DomainId: world.Domain, ConversationId: "cnv_1", Kind: "member", MemberId: "ana"}
	participant.OutboxFields = world.Staged(participant.OutboxFields, "read-position.moved", events.ReadPositionMoved{ConversationId: "cnv_1", MemberId: "ana"})
	id, err := store.Insert(participant)
	world.Must(err)
	Assert.Equals(1, world.Pushed("realtime", "read-position.moved"))
	inConversation, err := store.OfConversation(world.Domain, "cnv_1")
	Assert.IsTrue(err == nil && len(inConversation) == 1, "one participant")
	Assert.Equals(0, inConversation[0].OutboxPending)
	inConversation[0].ReadSequence = 4
	world.Must(store.Save(inConversation[0]))
	ofMember, _ := store.OfMember(world.Domain, "ana")
	Assert.Equals(4, ofMember[0].ReadSequence)
	world.Must(store.Delete("other-"+world.Domain, id))
	world.Must(store.Delete(world.Domain, id))
	left, _ := store.OfMember(world.Domain, "ana")
	Assert.Equals(0, len(left))
}

func (test *StorageTest) TestMessagesLiveInTheirMonth() {
	world := StorageTestSetup{}.World()
	store := world.Messages()
	id, err := store.Insert(core.Message{DomainId: world.Domain, ConversationId: "cnv_1", Month: "202611", SendKey: "send1", Body: "hi", Sequence: 1})
	world.Must(err)
	message, found, err := store.Find(world.Domain, "202611", id)
	Assert.IsTrue(err == nil && found, "found in its month")
	Assert.Equals("hi", message.Body)
	_, found, _ = store.Find(world.Domain, "202610", id)
	Assert.IsTrue(!found, "not found in another month")
	message.EventsStaged = true
	world.Must(store.Save(message))
	bySendKey, found, _ := store.BySendKey(world.Domain, "202611", "send1")
	Assert.IsTrue(found && bySendKey.EventsStaged, "found by send key")
	listed, _ := store.OfConversation(world.Domain, "202611", "cnv_1")
	Assert.Equals(1, len(listed))
	Assert.Equals(id, listed[0].Id)
}

func (test *StorageTest) TestReactionsAreFoundByKey() {
	world := StorageTestSetup{}.World()
	store := world.Reactions()
	_, err := store.Insert(core.Reaction{DomainId: world.Domain, Key: "r1", ConversationId: "cnv_1", Emoji: "+1", Active: true})
	world.Must(err)
	reaction, found, _ := store.ByKey(world.Domain, "r1")
	Assert.IsTrue(found, "found by key")
	reaction.Active = false
	world.Must(store.Save(reaction))
	listed, _ := store.OfConversation(world.Domain, "cnv_1")
	Assert.Equals(1, len(listed))
	Assert.IsTrue(!listed[0].Active, "saved")
}

func (test *StorageTest) TestThreadKeysMentionsAndMembers() {
	world := StorageTestSetup{}.World()
	_, err := world.ThreadKeys().Insert(core.ThreadKey{DomainId: world.Domain, Key: "k1", MessageId: "msg_1"})
	world.Must(err)
	threadKey, found, _ := world.ThreadKeys().ByKey(world.Domain, "k1")
	Assert.IsTrue(found, "thread key found")
	Assert.Equals("msg_1", threadKey.MessageId)
	_, err = world.Mentions().Insert(core.Mention{DomainId: world.Domain, MemberId: "ana", Sequence: 2})
	world.Must(err)
	mentions, _ := world.Mentions().OfMember(world.Domain, "ana")
	Assert.Equals(1, len(mentions))
	members := world.Members()
	world.Must(members.Save(core.Member{DomainId: world.Domain, MemberId: "ana", LocalPart: "ana", DisplayName: "Ana"}))
	world.Must(members.Save(core.Member{DomainId: world.Domain, MemberId: "ana", LocalPart: "ana", DisplayName: "Ana B"}))
	member, found, _ := members.ByLocalPart(world.Domain, "ana")
	Assert.IsTrue(found, "member found")
	Assert.Equals("Ana B", member.DisplayName)
	again, _, _ := members.ByMemberId(world.Domain, "ana")
	Assert.Equals(member.Id, again.Id)
}

func (test *StorageTest) TestAttachmentUsesAreFoundByAttachment() {
	world := StorageTestSetup{}.World()
	uses := world.AttachmentUses()
	_, err := uses.Insert(core.AttachmentUse{DomainId: world.Domain, AttachmentId: "att_1", ConversationId: "cnv_1", MessageId: "msg_1"})
	world.Must(err)
	_, err = uses.Insert(core.AttachmentUse{DomainId: world.Domain, AttachmentId: "att_2", ConversationId: "cnv_1", MessageId: "msg_2"})
	world.Must(err)
	found, err := uses.ByAttachment(world.Domain, "att_1")
	Assert.IsTrue(err == nil, "reading the uses failed")
	Assert.Equals(1, len(found))
	Assert.Equals("msg_1", found[0].MessageId)
	elsewhere, _ := uses.ByAttachment("other-"+world.Domain, "att_1")
	Assert.Equals(0, len(elsewhere))
}

func (test *StorageTest) TestSeenEventsAreRememberedPerDomain() {
	world := StorageTestSetup{}.World()
	seen := world.Seen()
	before, err := seen.Seen(world.Domain, "evt_1")
	Assert.IsTrue(err == nil && !before, "not seen yet")
	world.Must(seen.Remember(world.Domain, "evt_1"))
	after, _ := seen.Seen(world.Domain, "evt_1")
	Assert.IsTrue(after, "seen")
	elsewhere, _ := seen.Seen("other-"+world.Domain, "evt_1")
	Assert.IsTrue(!elsewhere, "not seen in another domain")
}
