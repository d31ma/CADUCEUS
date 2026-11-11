import (
	"errors"

	"mesail.dev/packages/core"
)

// @Test(MessagingTestSetup, {"partition":"tests"})
type MessagingServiceTest struct {
	behaviors *BehaviorContext
}

func (test *MessagingServiceTest) TestSendThenConversationsAndMessages() {
	world := MessagingTestSetup{}.World()
	request := core.SendRequest{ClientMessageId: "c1", Body: "hello", Recipients: []core.RecipientChoice{{Address: world.Address("bo"), Role: "to", Delivery: "chat", ExpectedKind: "member"}}}
	outcome, sendError := world.Service.Send(world.Member("ana"), world.MemberBearer("ana"), request)
	list, listError := world.Service.Conversations(world.Member("bo"), core.PageRequest{})
	messages, messagesError := world.Service.Messages(world.Member("bo"), MessageQuery{ConversationId: outcome.Message.Message.ConversationId})
	Assert.IsTrue(sendError == nil && listError == nil && messagesError == nil, "a call failed")
	Assert.Equals(1, len(list.Items))
	Assert.Equals("hello", messages.Items[0].Message.Body)
}

func (test *MessagingServiceTest) TestMoveReadPositionClearsUnread() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	err := world.Service.MoveReadPosition(world.Member("bo"), sent.Message.ConversationId, sent.Message.MessageId)
	list, _ := world.Service.Conversations(world.Member("bo"), core.PageRequest{})
	Assert.IsTrue(err == nil, "moving the read position failed")
	Assert.Equals(0, list.Items[0].UnreadCount)
}

func (test *MessagingServiceTest) TestEditAndDelete() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "helo")
	view, editError := world.Service.Edit(world.Member("ana"), sent.Message.MessageId, "hello")
	deleteError := world.Service.Delete(world.Member("ana"), sent.Message.MessageId)
	Assert.IsTrue(editError == nil && deleteError == nil, "a change failed")
	Assert.Equals("hello", view.Message.Body)
}

func (test *MessagingServiceTest) TestAddAndRemoveReaction() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	addError := world.Service.AddReaction(world.Member("bo"), sent.Message.MessageId, "thumbs_up")
	removeError := world.Service.RemoveReaction(world.Member("bo"), sent.Message.MessageId, "thumbs_up")
	page := world.MessagesOf(sent.Message.ConversationId, world.MemberBearer("bo"))
	Assert.IsTrue(addError == nil && removeError == nil, "a reaction call failed")
	Assert.Equals(map[string]int{}, page.Items[0].Reactions)
}

func (test *MessagingServiceTest) TestMessageAnswersOneMessage() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	view, err := world.Service.Message(world.Member("bo"), sent.Message.MessageId)
	Assert.IsTrue(err == nil, "reading the message failed")
	Assert.Equals("hello", view.Message.Body)
}

func (test *MessagingServiceTest) TestAttachmentAccessFollowsTheMessage() {
	world := MessagingTestSetup{}.World()
	world.AddMember("cy")
	world.WithAttachment("bo", "att_1")
	allowed := world.Service.AttachmentAccess(world.Member("bo"), "att_1")
	refused := world.Service.AttachmentAccess(world.Member("cy"), "att_1")
	Assert.IsTrue(allowed == nil, "the reader was refused")
	Assert.IsTrue(world.IsNotFound(refused), "want NotFound")
}

func (test *MessagingServiceTest) TestOriginalOfAChatMessageIsNotFound() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	_, err := world.Service.Original(world.Member("bo"), sent.Message.MessageId)
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}

func (test *MessagingServiceTest) TestSourceAnswersTheBody() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	source, err := world.Service.Source(world.Member("bo"), sent.Message.MessageId)
	Assert.IsTrue(err == nil, "reading the source failed")
	Assert.Equals("hello", source.Body)
}

func (test *MessagingServiceTest) TestADirectoryFailureAnswersUnavailable() {
	world := MessagingTestSetup{}.World()
	world.Fakes.Directory.Failure = errors.New("directory down")
	request := core.SendRequest{ClientMessageId: "c1", Body: "hello", Recipients: []core.RecipientChoice{{Address: world.Address("bo"), Role: "to", Delivery: "chat", ExpectedKind: "member"}}}
	_, err := world.Service.Send(world.Member("ana"), world.MemberBearer("ana"), request)
	Assert.IsTrue(world.IsUnavailable(err), "want Unavailable")
}
