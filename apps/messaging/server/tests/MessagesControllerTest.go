// @Test(MessagingTestSetup, {"partition":"tests"})
type MessagesControllerTest struct {
	behaviors *BehaviorContext
}

func (test *MessagesControllerTest) TestPostMessageAnswersChatAndEmailDeliveries() {
	world := MessagingTestSetup{}.World()
	request := SendRequest{ClientMessageId: "c1", Body: "Kickoff", Email: &EmailOptions{Subject: "Kickoff"}, Recipients: []RecipientChoice{
		{Address: world.Address("bo"), Role: "to", Delivery: "chat", ExpectedKind: "member"},
		{Address: "carol@partner.org", Role: "to", Delivery: "email", ExpectedKind: "external"},
	}}
	result, err := world.Messages.PostMessage(world.MemberBearer("ana"), request)
	Assert.IsTrue(err == nil, "the send failed")
	Assert.Equals([]string{world.Address("bo") + "/chat", "carol@partner.org/email"}, world.Channels(result.Deliveries))
	Assert.Equals("Kickoff", result.Message.Body)
	Assert.Equals("markdown", result.Message.BodyFormat)
	Assert.Equals("Kickoff", result.Message.Email.Subject)
	Assert.Equals("ana", result.Message.Author.MemberId)
}

func (test *MessagesControllerTest) TestPostMessageAnswersRecipientsChanged() {
	world := MessagingTestSetup{}.World()
	request := SendRequest{ClientMessageId: "c1", Body: "hi", Recipients: []RecipientChoice{{Address: "carol@partner.org", Role: "to", Delivery: "chat", ExpectedKind: "member"}}}
	_, err := world.Messages.PostMessage(world.MemberBearer("ana"), request)
	body := (&MessagingErrorHandler{}).RecipientsChanged(world.Changed(err))
	Assert.Equals("recipients_changed", body.Code)
	Assert.Equals([]string{"carol@partner.org"}, body.Addresses)
}

func (test *MessagesControllerTest) TestPostMessageAnswersChatNotAllowed() {
	world := MessagingTestSetup{}.World()
	request := SendRequest{ClientMessageId: "c1", Body: "hi", Recipients: []RecipientChoice{{Address: "carol@partner.org", Role: "to", Delivery: "chat", ExpectedKind: "external"}}}
	_, err := world.Messages.PostMessage(world.MemberBearer("ana"), request)
	body := (&MessagingErrorHandler{}).ChatNotAllowed(world.NotAllowed(err))
	Assert.Equals("chat_not_allowed", body.Code)
	Assert.Equals("carol@partner.org", body.Field)
}

func (test *MessagesControllerTest) TestPostMessageToAnotherDomainsConversationIsNotFound() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	request := SendRequest{ClientMessageId: "c2", ConversationId: sent.Message.ConversationId, Body: "hi"}
	_, err := world.Messages.PostMessage(world.StrangerBearer("ana"), request)
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}

func (test *MessagesControllerTest) TestPatchMessageChangesTheBody() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "helo")
	message, err := world.Messages.PatchMessage(world.MemberBearer("ana"), sent.Message.MessageId, EditRequest{Body: "hello"})
	Assert.IsTrue(err == nil, "the edit failed")
	Assert.Equals("hello", message.Body)
	Assert.IsTrue(message.EditedAt != "", "editedAt is missing")
}

func (test *MessagesControllerTest) TestDeleteMessageMarksTheMessageDeleted() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "oops")
	err := world.Messages.DeleteMessage(world.MemberBearer("ana"), sent.Message.MessageId)
	page := world.MessagesOf(sent.Message.ConversationId, world.MemberBearer("ana"))
	Assert.IsTrue(err == nil, "the delete failed")
	Assert.IsTrue(page.Items[0].DeletedAt != "", "deletedAt is missing")
}

func (test *MessagesControllerTest) TestPutReactionCountsTheReactionName() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	err := world.Messages.PutReaction(world.MemberBearer("bo"), sent.Message.MessageId, "thumbs_up")
	page := world.MessagesOf(sent.Message.ConversationId, world.MemberBearer("bo"))
	Assert.IsTrue(err == nil, "the reaction failed")
	Assert.Equals(map[string]int{"thumbs_up": 1}, page.Items[0].Reactions)
	Assert.Equals([]string{"thumbs_up"}, page.Items[0].MyReactions)
}

func (test *MessagesControllerTest) TestDeleteReactionTakesTheReactionBack() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	world.Must(world.Messages.PutReaction(world.MemberBearer("bo"), sent.Message.MessageId, "heart"))
	err := world.Messages.DeleteReaction(world.MemberBearer("bo"), sent.Message.MessageId, "heart")
	page := world.MessagesOf(sent.Message.ConversationId, world.MemberBearer("bo"))
	Assert.IsTrue(err == nil, "taking the reaction back failed")
	Assert.Equals(map[string]int{}, page.Items[0].Reactions)
	Assert.Equals([]string{}, page.Items[0].MyReactions)
}

func (test *MessagesControllerTest) TestPutReactionRefusesAnUnknownName() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	err := world.Messages.PutReaction(world.MemberBearer("bo"), sent.Message.MessageId, "unicorn")
	Assert.Equals("reaction", world.InvalidField(err))
}

func (test *MessagesControllerTest) TestGetMessageAnswersOneMessage() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	message, err := world.Messages.GetMessage(world.MemberBearer("bo"), sent.Message.MessageId)
	Assert.IsTrue(err == nil, "reading the message failed")
	Assert.Equals(sent.Message.MessageId, message.MessageId)
	Assert.Equals("hello", message.Body)
}

func (test *MessagesControllerTest) TestGetMessageHidesTheMessageFromOthers() {
	world := MessagingTestSetup{}.World()
	world.AddMember("cy")
	sent := world.ChatTo("bo", "c1", "hello")
	_, err := world.Messages.GetMessage(world.MemberBearer("cy"), sent.Message.MessageId)
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}

func (test *MessagesControllerTest) TestAReplyAlsoSentToTheConversationIsListedThere() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "root")
	request := SendRequest{ClientMessageId: "c2", ConversationId: sent.Message.ConversationId, ThreadRootId: sent.Message.MessageId, AlsoToConversation: true, Body: "reply"}
	reply, err := world.Messages.PostMessage(world.MemberBearer("ana"), request)
	page := world.MessagesOf(sent.Message.ConversationId, world.MemberBearer("bo"))
	Assert.IsTrue(err == nil, "the reply failed")
	Assert.IsTrue(reply.Message.AlsoToConversation, "the flag is missing")
	Assert.Equals(2, len(page.Items))
	Assert.Equals(reply.Message.MessageId, page.Items[0].MessageId)
}

func (test *MessagesControllerTest) TestAlsoToConversationNeedsAThread() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "root")
	request := SendRequest{ClientMessageId: "c2", ConversationId: sent.Message.ConversationId, AlsoToConversation: true, Body: "reply"}
	_, err := world.Messages.PostMessage(world.MemberBearer("ana"), request)
	Assert.Equals("alsoToConversation", world.InvalidField(err))
}

func (test *MessagesControllerTest) TestGetOriginalOfAChatMessageIsNotFound() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	_, err := world.Messages.GetOriginal(world.MemberBearer("bo"), sent.Message.MessageId)
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}

func (test *MessagesControllerTest) TestEveryRouteRefusesAMissingToken() {
	world := MessagingTestSetup{}.World()
	_, sendError := world.Messages.PostMessage("", SendRequest{ClientMessageId: "c1", Body: "hi"})
	_, editError := world.Messages.PatchMessage("", "msg_1", EditRequest{Body: "hi"})
	deleteError := world.Messages.DeleteMessage("", "msg_1")
	putError := world.Messages.PutReaction("", "msg_1", "heart")
	takeBackError := world.Messages.DeleteReaction("", "msg_1", "heart")
	_, originalError := world.Messages.GetOriginal("", "msg_1")
	_, messageError := world.Messages.GetMessage("", "msg_1")
	Assert.IsTrue(world.IsUnauthenticated(sendError), "send: want Unauthenticated")
	Assert.IsTrue(world.IsUnauthenticated(editError), "edit: want Unauthenticated")
	Assert.IsTrue(world.IsUnauthenticated(deleteError), "delete: want Unauthenticated")
	Assert.IsTrue(world.IsUnauthenticated(putError), "react: want Unauthenticated")
	Assert.IsTrue(world.IsUnauthenticated(takeBackError), "take back: want Unauthenticated")
	Assert.IsTrue(world.IsUnauthenticated(originalError), "original: want Unauthenticated")
	Assert.IsTrue(world.IsUnauthenticated(messageError), "message: want Unauthenticated")
}

func (test *MessagesControllerTest) TestAServiceTokenIsForbiddenOnChatRoutes() {
	world := MessagingTestSetup{}.World()
	_, err := world.Messages.PostMessage(world.ServiceBearer(world.Home), SendRequest{ClientMessageId: "c1", Body: "hi"})
	Assert.IsTrue(world.IsForbidden(err), "want Forbidden")
}

func (test *MessagesControllerTest) TestPostMessageAnswersTheAttachmentDetails() {
	world := MessagingTestSetup{}.World()
	result := world.WithAttachment("bo", "att_1")
	Assert.Equals([]AttachmentRef{{AttachmentId: "att_1", FileName: "att_1.txt", MediaType: "application/octet-stream", Size: 100}}, result.Message.Attachments)
}

func (test *MessagesControllerTest) TestPostMessageWithAnUnknownAttachmentIsInvalid() {
	world := MessagingTestSetup{}.World()
	request := SendRequest{ClientMessageId: "c1", Body: "file", AttachmentIds: []string{"att_missing"}, Recipients: []RecipientChoice{{Address: world.Address("bo"), Role: "to", Delivery: "chat", ExpectedKind: "member"}}}
	_, err := world.Messages.PostMessage(world.MemberBearer("ana"), request)
	Assert.Equals("attachmentIds", world.InvalidField(err))
}

func (test *MessagesControllerTest) TestPostMessageWithTooLargeAttachmentsAnswersTooLarge() {
	world := MessagingTestSetup{}.World()
	world.Fakes.Attachments.Ready(world.Domain, "att_1", "a.bin", 30<<20)
	world.Fakes.Attachments.Ready(world.Domain, "att_2", "b.bin", 30<<20)
	request := SendRequest{ClientMessageId: "c1", Body: "files", AttachmentIds: []string{"att_1", "att_2"}, Recipients: []RecipientChoice{{Address: world.Address("bo"), Role: "to", Delivery: "chat", ExpectedKind: "member"}}}
	_, err := world.Messages.PostMessage(world.MemberBearer("ana"), request)
	Assert.IsTrue(world.IsTooLarge(err), "the send must answer too_large")
}
