// @Test(ThreadTestSetup, {"partition":"tests"})
type ThreadRowServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ThreadRowServiceTest) message() ThreadMessage {
	author := ThreadAuthor{Kind: "member", Address: "ana@example.com", DisplayName: "Ana Maria Ruiz"}
	return ThreadMessage{MessageId: "m-1", Author: author, Body: "Hello", CreatedAt: "2026-11-10T09:12:30.250Z"}
}

func (test *ThreadRowServiceTest) TestRowShowsTheAuthorAndTime() {
	row := (&ThreadRowService{}).Row(test.message())
	Assert.Equals("AM", row.Initials)
	Assert.Equals("Ana Maria Ruiz", row.AuthorName)
	Assert.Equals("09:12", row.Time)
	Assert.Equals("2026-11-10T09:12:30.250Z", row.DateTime)
	Assert.Equals("Ana Maria Ruiz, 09:12", row.Label)
	Assert.Equals("Hello", row.Body)
}

func (test *ThreadRowServiceTest) TestRowShowsTimeInUtc() {
	message := test.message()
	message.CreatedAt = "2026-11-10T09:12:00+02:00"
	Assert.Equals("07:12", (&ThreadRowService{}).Row(message).Time)
}

func (test *ThreadRowServiceTest) TestRowUsesTheAddressWithoutAName() {
	message := test.message()
	message.Author.DisplayName = ""
	row := (&ThreadRowService{}).Row(message)
	Assert.Equals("ana@example.com", row.AuthorName)
	Assert.Equals("A", row.Initials)
}

func (test *ThreadRowServiceTest) TestRowWithoutATimeLabelsOnlyTheName() {
	message := test.message()
	message.CreatedAt = ""
	row := (&ThreadRowService{}).Row(message)
	Assert.Equals("", row.Time)
	Assert.Equals("Ana Maria Ruiz", row.Label)
}

func (test *ThreadRowServiceTest) TestRowMarksEditedAndDeleted() {
	message := test.message()
	message.EditedAt = "2026-11-10T09:13:00Z"
	message.DeletedAt = "2026-11-10T09:14:00Z"
	row := (&ThreadRowService{}).Row(message)
	Assert.IsTrue(row.Edited)
	Assert.IsTrue(row.Deleted)
}

func (test *ThreadRowServiceTest) TestRowSummarizesReactionsInOrder() {
	message := test.message()
	message.Reactions = map[string]int{"tada": 1, "+1": 2}
	Assert.Equals("+1 2, tada 1", (&ThreadRowService{}).Row(message).Reactions)
}

func (test *ThreadRowServiceTest) TestRepliesShowOldestFirst() {
	newer := ThreadMessage{MessageId: "m-3"}
	older := ThreadMessage{MessageId: "m-2"}
	replies := (&ThreadRowService{}).Replies([]ThreadMessage{newer, older})
	Assert.Equals(2, len(replies))
	Assert.Equals("m-2", replies[0].MessageId)
	Assert.Equals("m-3", replies[1].MessageId)
}

func (test *ThreadRowServiceTest) TestRepliesLabelForOneReply() {
	Assert.Equals("1 reply", (&ThreadRowService{}).RepliesLabel(ThreadMessage{}, 1))
}

func (test *ThreadRowServiceTest) TestRepliesLabelForNoReplies() {
	Assert.Equals("0 replies", (&ThreadRowService{}).RepliesLabel(ThreadMessage{}, 0))
}

func (test *ThreadRowServiceTest) TestRepliesLabelTrustsTheRootCountWhenOlderRepliesAreHidden() {
	Assert.Equals("5 replies", (&ThreadRowService{}).RepliesLabel(ThreadMessage{ReplyCount: 5}, 2))
}

func (test *ThreadRowServiceTest) TestPendingRowShowsMyNameAndText() {
	row := (&ThreadRowService{}).PendingRow("Ana Ruiz", "Thanks")
	Assert.Equals("AR", row.Initials)
	Assert.Equals("Thanks", row.Body)
	Assert.Equals("Ana Ruiz", row.Label)
}

func (test *ThreadRowServiceTest) TestConversationOfReadsTheConversation() {
	Assert.Equals("conv-1", (&ThreadRowService{}).ConversationOf(ThreadMessage{ConversationId: "conv-1"}))
}

func (test *ThreadRowServiceTest) TestRootIdOfARootIsItsOwnId() {
	Assert.Equals("m-1", (&ThreadRowService{}).RootIdOf(test.message()))
}

func (test *ThreadRowServiceTest) TestRootIdOfAReplyIsItsRoot() {
	Assert.Equals("m-1", (&ThreadRowService{}).RootIdOf(ThreadMessage{MessageId: "m-2", ThreadRootId: "m-1"}))
}

func (test *ThreadRowServiceTest) TestKnownRootKeepsARoot() {
	Assert.Equals("m-1", (&ThreadRowService{}).KnownRoot(test.message()).MessageId)
}

func (test *ThreadRowServiceTest) TestKnownRootDropsAReply() {
	Assert.Equals("", (&ThreadRowService{}).KnownRoot(ThreadMessage{MessageId: "m-2", ThreadRootId: "m-1"}).MessageId)
}
