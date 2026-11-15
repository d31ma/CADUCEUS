import "mesail.dev/packages/core"

// @Test(MessagingTestSetup, {"partition":"tests"})
type SyncTest struct {
	behaviors *BehaviorContext
}

func (test *SyncTest) TestPatchMessageWithAnOldVersionAnswersEditConflict() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	edited, err := world.Messages.PatchMessage(world.MemberBearer("ana"), sent.Message.MessageId, EditRequest{Body: "from the laptop", IfVersion: 1})
	Assert.IsTrue(err == nil, "the first edit failed")
	Assert.Equals(2, edited.Version)
	_, err = world.Messages.PatchMessage(world.MemberBearer("ana"), sent.Message.MessageId, EditRequest{Body: "from the phone", IfVersion: 1})
	body := (&MessagingErrorHandler{}).EditConflict(world.EditConflictOf(err))
	Assert.Equals("edit_conflict", body.Code)
	Assert.Equals("[MS1563]: the message changed since you read it", body.Message)
	Assert.Equals("from the laptop", body.Current.Body)
	Assert.Equals(2, body.Current.Version)
}

func (test *SyncTest) TestReactionsOnADeletedMessageAnswerMessageDeleted() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	world.Must(world.Messages.DeleteMessage(world.MemberBearer("ana"), sent.Message.MessageId))
	world.Must(world.Messages.DeleteMessage(world.MemberBearer("ana"), sent.Message.MessageId))
	err := world.Messages.PutReaction(world.MemberBearer("bo"), sent.Message.MessageId, "heart")
	body := (&MessagingErrorHandler{}).Conflict(world.ConflictOf(err))
	Assert.Equals(ApiError{Code: "message_deleted", Message: "[MS1545]: the message is deleted"}, body)
}

func (test *SyncTest) TestPostMessageKeepsAnEarlierWrittenAt() {
	world := MessagingTestSetup{}.World()
	written := world.Fakes.Clock.Now().Add(-3600 * 1e9).Format("2006-01-02T15:04:05Z07:00")
	request := SendRequest{ClientMessageId: "w1", Body: "offline", WrittenAt: written, Recipients: []RecipientChoice{{Address: world.Address("bo"), Role: "to", Delivery: "chat", ExpectedKind: "member"}}}
	result, err := world.Messages.PostMessage(world.MemberBearer("ana"), request)
	Assert.IsTrue(err == nil, "the send failed")
	Assert.Equals(written, result.Message.WrittenAt)
}

func (test *SyncTest) TestTheListNamesTheNewestChangeAndTheReadPosition() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	world.LogCreated(sent)
	world.Must(world.Conversations.PutRead(world.MemberBearer("bo"), sent.Message.ConversationId, ReadRequest{MessageId: sent.Message.MessageId}))
	page, err := world.Conversations.GetConversations(world.Request("/api/v1/messaging/conversations", world.MemberBearer("bo")))
	Assert.IsTrue(err == nil, "the list failed")
	head, _, _ := world.Fakes.Stores.ChangeHeads.Find(world.Domain, sent.Message.ConversationId)
	Assert.Equals(head.LastChangeId, page.Items[0].LastChangeId)
	Assert.Equals(sent.Message.MessageId, page.Items[0].ReadMessageId)
}

func (test *SyncTest) TestMessagingServiceSyncAnswersEachCursor() {
	world := MessagingTestSetup{}.World()
	answers, err := world.Service.Sync(world.Member("ana"), []core.SyncCursor{{ConversationId: "cnv_unknown"}})
	Assert.IsTrue(err == nil, "the sync failed")
	Assert.Equals("gone", answers[0].State)
}

func (test *SyncTest) TestWireServiceCopiesSyncCursorsAndAnswers() {
	wire := &WireService{}
	cursors := wire.SyncCursors(SyncRequest{Cursors: []SyncCursor{{ConversationId: "cnv_1", After: "a"}}})
	Assert.Equals([]core.SyncCursor{{ConversationId: "cnv_1", After: "a"}}, cursors)
	response := wire.SyncResponse([]core.ConversationSync{{ConversationId: "cnv_1", State: "unchanged", Next: "a"}})
	Assert.Equals(SyncResponse{Conversations: []ConversationSync{{ConversationId: "cnv_1", State: "unchanged", Messages: []Message{}, Next: "a"}}}, response)
}

func (test *SyncTest) TestFailureServiceAnswersAnEditConflictWithTheCurrentMessage() {
	view := core.MessageView{Message: core.Message{Id: "1", Month: "202611", Body: "now", Version: 3}}
	known := (&FailureService{wire: &WireService{}}).Known(core.EditConflict{}.Of(view)).(*ChangedMessage)
	Assert.Equals(1563, known.ErrorNumber)
	Assert.Equals("now", known.Current.Body)
	Assert.Equals(3, known.Current.Version)
}
