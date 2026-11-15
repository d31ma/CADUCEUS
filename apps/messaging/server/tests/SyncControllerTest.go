import "time"

// @Test(MessagingTestSetup, {"partition":"tests"})
type SyncControllerTest struct {
	behaviors *BehaviorContext
}

func (test *SyncControllerTest) TestPostSyncAnswersTheChangedMessages() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	world.LogCreated(sent)
	request := SyncRequest{Cursors: []SyncCursor{{ConversationId: sent.Message.ConversationId, After: world.CursorBefore(time.Hour)}}}
	response, err := world.Sync.PostSync(world.MemberBearer("bo"), request)
	Assert.IsTrue(err == nil, "the sync failed")
	answer := response.Conversations[0]
	Assert.Equals("changed", answer.State)
	Assert.Equals("hello", answer.Messages[0].Body)
	Assert.Equals(1, answer.Messages[0].Version)
	Assert.IsTrue(answer.Next != "" && !answer.More, "want the newest cursor")
}

func (test *SyncControllerTest) TestPostSyncAnswersResetThenUnchanged() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	world.LogCreated(sent)
	conversationId := sent.Message.ConversationId
	reset, err := world.Sync.PostSync(world.MemberBearer("ana"), SyncRequest{Cursors: []SyncCursor{{ConversationId: conversationId}}})
	Assert.IsTrue(err == nil, "the sync failed")
	Assert.Equals("reset", reset.Conversations[0].State)
	unchanged, err := world.Sync.PostSync(world.MemberBearer("ana"), SyncRequest{Cursors: []SyncCursor{{ConversationId: conversationId, After: reset.Conversations[0].Next}}})
	Assert.IsTrue(err == nil, "the second sync failed")
	Assert.Equals("unchanged", unchanged.Conversations[0].State)
	Assert.Equals(0, len(unchanged.Conversations[0].Messages))
}

func (test *SyncControllerTest) TestPostSyncAnswersGoneForAnotherDomain() {
	world := MessagingTestSetup{}.World()
	sent := world.ChatTo("bo", "c1", "hello")
	response, err := world.Sync.PostSync(world.StrangerBearer("ana"), SyncRequest{Cursors: []SyncCursor{{ConversationId: sent.Message.ConversationId}, {ConversationId: "cnv_unknown"}}})
	Assert.IsTrue(err == nil, "the sync failed")
	Assert.Equals("gone", response.Conversations[0].State)
	Assert.Equals("gone", response.Conversations[1].State)
}

func (test *SyncControllerTest) TestPostSyncRefusesMoreThanFiftyCursors() {
	world := MessagingTestSetup{}.World()
	_, err := world.Sync.PostSync(world.MemberBearer("ana"), SyncRequest{Cursors: make([]SyncCursor, 51)})
	Assert.Equals("cursors", world.InvalidField(err))
	Assert.Equals(1564, world.Number(err))
}

func (test *SyncControllerTest) TestPostSyncRefusesAnUnsignedCaller() {
	world := MessagingTestSetup{}.World()
	_, err := world.Sync.PostSync("Bearer nonsense", SyncRequest{Cursors: []SyncCursor{{ConversationId: "cnv_1"}}})
	Assert.IsTrue(world.IsUnauthenticated(err), "want Unauthenticated")
}
