import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/fakes"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

// @Test(StorageTestSetup, {"partition":"tests"})
type ChannelReadersTest struct {
	behaviors *BehaviorContext
}

func (test *ChannelReadersTest) TestAPostInAChannelOfAThousandMembersKeepsEveryDocumentSmall() {
	world := StorageTestSetup{}.World()
	service := test.service(world)
	test.down(world, "realtime", "notify", "search")
	conversation := test.channel(world, 1000)
	messageId := test.posted(world, service, "@m00042 the build is green")
	_, editError := service.Edit(test.member(world, "ana"), messageId, "@m00042 the build is green again")
	reactionError := service.AddReaction(test.member(world, "m00007"), messageId, "thumbs_up")
	message := test.stored(world, messageId)
	created := test.created(world, message)
	Assert.IsTrue(editError == nil && reactionError == nil, fmt.Sprintf("edit %v, reaction %v", editError, reactionError))
	Assert.Equals("channel", created.ConversationKind)
	Assert.Equals(0, len(created.MemberIds))
	Assert.Equals([]string{"m00042"}, created.MentionedMemberIds)
	test.small(world, "the message", message)
	test.small(world, "the conversation", test.conversation(world, conversation))
	reactions := test.reactions(world, conversation)
	Assert.Equals([]int{0}, test.readerCounts(world, message.OutboxFields, "message.edited"))
	Assert.Equals([]int{0}, test.readerCounts(world, reactions[0].OutboxFields, "reaction.changed"))
	test.small(world, "the reactions", reactions)
}

func (test *ChannelReadersTest) service(world *StorageTestSetup) *MessagingService {
	fake := fakes.NewFakeWorld()
	settings, err := world.Settings.Outbox()
	world.Must(err)
	ports := fake.Ports
	ports.Stager = core.NewStager(settings, time.Now, rand.Reader)
	fake.Directory.Channel("all-hands@"+world.Domain, "chan1")
	coreService := &CoreService{storage: world.Storage, settings: world.Settings, fixedPorts: &ports}
	return &MessagingService{core: coreService, failures: &FailureService{}}
}

func (test *ChannelReadersTest) channel(world *StorageTestSetup, memberCount int) core.Conversation {
	conversation := core.Conversation{DomainId: world.Domain, Kind: "channel", Key: "chan1", ChannelId: "chan1", Visibility: "public", EmailAddress: "all-hands@" + world.Domain, CreatedMonth: "202611", CreatedAt: "2026-11-12T00:00:00Z"}
	id, err := world.Conversations().Insert(conversation)
	world.Must(err)
	conversation.Id = id
	test.joined(world, core.ConversationIds{}.Wire(conversation), test.memberIds(memberCount))
	return conversation
}

func (test *ChannelReadersTest) memberIds(memberCount int) []string {
	memberIds := []string{"ana"}
	for index := 1; index < memberCount; index++ {
		memberIds = append(memberIds, fmt.Sprintf("m%05d", index))
	}
	return memberIds
}

func (test *ChannelReadersTest) joined(world *StorageTestSetup, conversationId string, memberIds []string) {
	collection := world.Names.Participants(world.Domain)
	_, err := world.Participants().Insert(test.participant(world, conversationId, memberIds[0]))
	world.Must(err)
	work := make(chan string)
	failures := make(chan error, len(memberIds))
	group := sync.WaitGroup{}
	for range test.writers() {
		group.Go(func() {
			for memberId := range work {
				_, err := DataSource{}.Insert(collection, test.participant(world, conversationId, memberId))
				failures <- err
			}
		})
	}
	for _, memberId := range memberIds[1:] {
		work <- memberId
	}
	close(work)
	group.Wait()
	close(failures)
	for failure := range failures {
		world.Must(failure)
	}
	for _, memberId := range []string{"ana", "m00007", "m00042"} {
		world.Must(world.Members().Save(core.Member{DomainId: world.Domain, MemberId: memberId, LocalPart: memberId, DisplayName: memberId, PrimaryAddress: memberId + "@" + world.Domain}))
	}
}

func (test *ChannelReadersTest) writers() int {
	return 4
}

func (test *ChannelReadersTest) participant(world *StorageTestSetup, conversationId string, memberId string) core.Participant {
	return core.Participant{DomainId: world.Domain, ConversationId: conversationId, Kind: "member", MemberId: memberId, Address: memberId + "@" + world.Domain, DisplayName: memberId, Role: "member"}
}

func (test *ChannelReadersTest) member(world *StorageTestSetup, memberId string) oidc.Caller {
	return oidc.Caller{DomainId: world.Domain, MemberId: memberId, DisplayName: memberId, Email: memberId + "@" + world.Domain, Roles: []string{"Member"}}
}

func (test *ChannelReadersTest) posted(world *StorageTestSetup, service *MessagingService, body string) string {
	recipient := core.RecipientChoice{Address: "all-hands@" + world.Domain, Role: "to", Delivery: "chat", ExpectedKind: "channel"}
	request := core.SendRequest{ClientMessageId: "c-" + world.RandomText(), Body: body, Recipients: []core.RecipientChoice{recipient}}
	outcome, err := service.Send(test.member(world, "ana"), "Bearer unused", request)
	world.Must(err)
	return core.MessageIds{}.Wire(outcome.Message.Message.Month, outcome.Message.Message.Id)
}

func (test *ChannelReadersTest) down(world *StorageTestSetup, subscribers ...string) {
	for _, subscriber := range subscribers {
		world.Pusher.FailFor(subscriber, outbox.DeliveryFailure{Subscriber: subscriber, Detail: "down"})
	}
}

func (test *ChannelReadersTest) stored(world *StorageTestSetup, messageId string) core.Message {
	month, storedId, err := core.MessageIds{}.Parse(messageId)
	world.Must(err)
	message, found, err := world.Messages().Find(world.Domain, month, storedId)
	world.Must(err)
	Assert.IsTrue(found, "the message must be stored")
	return message
}

func (test *ChannelReadersTest) created(world *StorageTestSetup, message core.Message) events.MessageCreated {
	envelopes, err := outbox.Published{}.Staged(message.OutboxFields.Fields(), "message.created")
	world.Must(err)
	Assert.Equals(1, len(envelopes))
	payload := events.MessageCreated{}
	world.Must(events.PayloadReader{}.Read(envelopes[0], &payload))
	return payload
}

func (test *ChannelReadersTest) conversation(world *StorageTestSetup, conversation core.Conversation) core.Conversation {
	found, _, err := world.Conversations().Find(world.Domain, conversation.Key)
	world.Must(err)
	return found
}

func (test *ChannelReadersTest) reactions(world *StorageTestSetup, conversation core.Conversation) []core.Reaction {
	found, err := world.Reactions().OfConversation(world.Domain, core.ConversationIds{}.Wire(conversation))
	world.Must(err)
	Assert.Equals(1, len(found))
	return found
}

func (test *ChannelReadersTest) small(world *StorageTestSetup, what string, document any) {
	encoded, err := json.Marshal(document)
	world.Must(err)
	Assert.IsTrue(len(encoded) < 512*1024, fmt.Sprintf("%s has %d bytes", what, len(encoded)))
}

func (test *ChannelReadersTest) readerCounts(world *StorageTestSetup, fields core.OutboxFields, eventType string) []int {
	envelopes, err := outbox.Published{}.Staged(fields.Fields(), eventType)
	world.Must(err)
	counts := []int{}
	for _, envelope := range envelopes {
		payload := struct {
			MemberIds []string `json:"memberIds"`
		}{}
		world.Must(json.Unmarshal(envelope.Payload, &payload))
		counts = append(counts, len(payload.MemberIds))
	}
	return counts
}
