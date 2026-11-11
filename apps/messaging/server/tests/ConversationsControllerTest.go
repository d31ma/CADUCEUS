// @Test(MessagingTestSetup, {"partition":"tests"})
type ConversationsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *ConversationsControllerTest) TestGetConversationsRefusesARequestWithoutAToken() {
	world := MessagingTestSetup{}.World()
	_, err := world.Conversations.GetConversations(world.Request("/api/v1/messaging/conversations", ""))
	Assert.IsTrue(world.IsUnauthenticated(err), "want Unauthenticated")
}

func (test *ConversationsControllerTest) TestGetConversationsListsTheConversationOfTheRecipient() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	page, err := world.Conversations.GetConversations(world.Request("/api/v1/messaging/conversations", world.MemberBearer("bo")))
	Assert.IsTrue(err == nil, "listing failed")
	Assert.Equals(1, len(page.Items))
	Assert.Equals(sent.Message.ConversationId, page.Items[0].ConversationId)
	Assert.Equals(1, page.Items[0].UnreadCount)
}

func (test *ConversationsControllerTest) TestGetConversationsRefusesALimitThatIsNotANumber() {
	world := MessagingTestSetup{}.World()
	_, err := world.Conversations.GetConversations(world.Request("/api/v1/messaging/conversations?limit=many", world.MemberBearer("bo")))
	Assert.Equals("limit", world.InvalidField(err))
}

func (test *ConversationsControllerTest) TestGetMessagesListsTheNewestFirst() {
	world := MessagingTestSetup{}.World()
	first := world.ChatTo("bo", "c1", "first")
	world.Reply(first.Message.ConversationId, "c2", "second")
	page := world.MessagesOf(first.Message.ConversationId, world.MemberBearer("bo"))
	Assert.Equals(2, len(page.Items))
	Assert.Equals("second", page.Items[0].Body)
	Assert.Equals("first", page.Items[1].Body)
}

func (test *ConversationsControllerTest) TestGetMessagesOfAnotherDomainIsNotFound() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	conversationId := sent.Message.ConversationId
	_, err := world.Conversations.GetMessages(conversationId, world.Request("/api/v1/messaging/conversations/"+conversationId+"/messages", world.StrangerBearer("bo")))
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}

func (test *ConversationsControllerTest) TestPutReadClearsTheUnreadCount() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	err := world.Conversations.PutRead(world.MemberBearer("bo"), sent.Message.ConversationId, ReadRequest{MessageId: sent.Message.MessageId})
	page, _ := world.Conversations.GetConversations(world.Request("/api/v1/messaging/conversations", world.MemberBearer("bo")))
	Assert.IsTrue(err == nil, "moving the read position failed")
	Assert.Equals(0, page.Items[0].UnreadCount)
}

func (test *ConversationsControllerTest) TestPutReadRefusesAForgedToken() {
	world := MessagingTestSetup{}.World()
	err := world.Conversations.PutRead("Bearer forged", "cnv_1", ReadRequest{MessageId: "msg_1"})
	Assert.IsTrue(world.IsUnauthenticated(err), "want Unauthenticated")
}
