// @Test(ThreadTestSetup, {"partition":"tests"})
type ThreadRowServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ThreadRowServiceTest) rows() *ThreadRowService {
	BrowserFakes{}.Answer("clock", "timeZone", "UTC")
	return &ThreadRowService{times: &ConversationTimeService{}, reactions: &ConversationReactionService{}}
}

func (test *ThreadRowServiceTest) message() Message {
	author := Author{Kind: "member", Address: "ana@example.com", DisplayName: "Ana Maria Ruiz"}
	return Message{MessageId: "m-1", Author: author, Body: "Hello", CreatedAt: "2026-11-10T09:12:30.250Z"}
}

func (test *ThreadRowServiceTest) TestRowShowsTheAuthorAndTime() {
	row := test.rows().Row(test.message())
	Assert.Equals("AM", row.Initials)
	Assert.Equals("Ana Maria Ruiz", row.AuthorName)
	Assert.Equals("09:12", row.Time)
	Assert.Equals("2026-11-10T09:12:30.250Z", row.DateTime)
	Assert.Equals("Ana Maria Ruiz, 09:12", row.Label)
	Assert.Equals("Hello", row.Body)
}

func (test *ThreadRowServiceTest) TestRowShowsTimeInTheBrowserTimeZone() {
	rows := test.rows()
	BrowserFakes{}.Answer("clock", "timeZone", "Europe/Berlin")
	Assert.Equals("10:12", rows.Row(test.message()).Time)
}

func (test *ThreadRowServiceTest) TestRowKeepsTheBlocksForTheBody() {
	message := test.message()
	message.Blocks = []Block{{Kind: "paragraph", Spans: []Span{{Text: "Hello", Bold: true}}}}
	message.Truncated = true
	row := test.rows().Row(message)
	Assert.Equals("paragraph", row.Blocks[0].Kind)
	Assert.Equals("Hello", row.Blocks[0].Spans[0].Text)
	Assert.IsTrue(row.Truncated)
}

func (test *ThreadRowServiceTest) TestRowUsesTheAddressWithoutAName() {
	message := test.message()
	message.Author.DisplayName = ""
	row := test.rows().Row(message)
	Assert.Equals("ana@example.com", row.AuthorName)
	Assert.Equals("A", row.Initials)
}

func (test *ThreadRowServiceTest) TestRowWithoutATimeLabelsOnlyTheName() {
	message := test.message()
	message.CreatedAt = ""
	row := test.rows().Row(message)
	Assert.Equals("", row.Time)
	Assert.Equals("Ana Maria Ruiz", row.Label)
}

func (test *ThreadRowServiceTest) TestRowMarksEditedAndDeleted() {
	message := test.message()
	message.EditedAt = "2026-11-10T09:13:00Z"
	message.DeletedAt = "2026-11-10T09:14:00Z"
	row := test.rows().Row(message)
	Assert.IsTrue(row.Edited)
	Assert.IsTrue(row.Deleted)
}

func (test *ThreadRowServiceTest) TestRowSummarizesReactionsInOrder() {
	message := test.message()
	message.Reactions = map[string]int{"party": 1, "thumbs_up": 2}
	message.MyReactions = []string{"party"}
	reactions := test.rows().Row(message).Reactions
	Assert.Equals(2, len(reactions))
	Assert.Equals("thumbs_up", reactions[0].Emoji)
	Assert.Equals(2, reactions[0].Count)
	Assert.IsTrue(reactions[1].Mine)
}

func (test *ThreadRowServiceTest) TestTimelineMarksTheRootFirst() {
	timeline := test.rows().Timeline(test.message(), []Message{{MessageId: "m-3"}, {MessageId: "m-2"}})
	Assert.Equals(3, len(timeline))
	Assert.IsTrue(timeline[0].IsRoot)
	Assert.Equals("m-2", timeline[1].MessageId)
}

func (test *ThreadRowServiceTest) TestTimelineWithoutARootShowsOnlyReplies() {
	timeline := test.rows().Timeline(Message{}, []Message{{MessageId: "m-2"}})
	Assert.Equals(1, len(timeline))
	Assert.IsTrue(!timeline[0].IsRoot)
}

func (test *ThreadRowServiceTest) TestRepliesShowOldestFirst() {
	newer := Message{MessageId: "m-3"}
	older := Message{MessageId: "m-2"}
	replies := test.rows().Replies([]Message{newer, older})
	Assert.Equals(2, len(replies))
	Assert.Equals("m-2", replies[0].MessageId)
	Assert.Equals("m-3", replies[1].MessageId)
}

func (test *ThreadRowServiceTest) TestRepliesLabelForOneReply() {
	Assert.Equals("1 reply", test.rows().RepliesLabel(Message{}, 1))
}

func (test *ThreadRowServiceTest) TestRepliesLabelForNoReplies() {
	Assert.Equals("0 replies", test.rows().RepliesLabel(Message{}, 0))
}

func (test *ThreadRowServiceTest) TestRepliesLabelTrustsTheRootCountWhenOlderRepliesAreHidden() {
	Assert.Equals("5 replies", test.rows().RepliesLabel(Message{ReplyCount: 5}, 2))
}

func (test *ThreadRowServiceTest) TestPendingRowShowsMyNameAndText() {
	row := test.rows().PendingRow("Ana Ruiz", "Thanks")
	Assert.Equals("AR", row.Initials)
	Assert.Equals("Thanks", row.Body)
	Assert.Equals("Ana Ruiz", row.Label)
}
