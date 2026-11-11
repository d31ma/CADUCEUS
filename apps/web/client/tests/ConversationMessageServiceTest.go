// @Test(ConversationTestSetup, {"partition":"tests"})
type ConversationMessageServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ConversationMessageServiceTest) service() *ConversationMessageService {
	return &ConversationMessageService{reactions: &ConversationReactionService{}}
}

func (test *ConversationMessageServiceTest) general() []Message {
	return ConversationTestSetup{}.GeneralMessages()
}

func (test *ConversationMessageServiceTest) TestCombineKeepsNewerFirstWithoutDuplicates() {
	older := append(test.general()[1:], ConversationTestSetup{}.OlderMessages()...)
	combined := test.service().Combine(test.general(), older)
	Assert.Equals(4, len(combined))
	Assert.Equals("m-3", combined[0].MessageId)
	Assert.Equals("m-0", combined[3].MessageId)
}

func (test *ConversationMessageServiceTest) TestCombineOfNothingIsEmpty() {
	Assert.Equals(0, len(test.service().Combine(nil, nil)))
}

func (test *ConversationMessageServiceTest) TestReplaceSwapsTheMessageWithTheSameId() {
	changed := test.general()[1]
	changed.Body = "Lunch at one?"
	replaced := test.service().Replace(test.general(), changed)
	Assert.Equals(3, len(replaced))
	Assert.Equals("Lunch at one?", replaced[1].Body)
	Assert.Equals("Thanks @ana", replaced[0].Body)
}

func (test *ConversationMessageServiceTest) TestFindGivesTheMessage() {
	Assert.Equals("Lunch at noon?", test.service().Find(test.general(), "m-2").Body)
}

func (test *ConversationMessageServiceTest) TestFindOfAMissingIdGivesAnEmptyMessage() {
	Assert.Equals("", test.service().Find(test.general(), "m-7").MessageId)
}

func (test *ConversationMessageServiceTest) TestNewestIsTheFirstMessage() {
	Assert.Equals("m-3", test.service().Newest(test.general()))
}

func (test *ConversationMessageServiceTest) TestNewestOfNothingIsEmpty() {
	Assert.Equals("", test.service().Newest(nil))
}

func (test *ConversationMessageServiceTest) TestSubjectComesFromTheNewestEmail() {
	messages := append(test.general(), ConversationTestSetup{}.EmailMessage())
	Assert.Equals("Quote", test.service().Subject(messages))
}

func (test *ConversationMessageServiceTest) TestSubjectWithoutEmailIsEmpty() {
	Assert.Equals("", test.service().Subject(test.general()))
}

func (test *ConversationMessageServiceTest) TestDeleteWarningNamesEmailRecipients() {
	warning := test.service().DeleteWarning(test.general()[1])
	Assert.Equals("carol@outside.test already got this as an email. Email cannot be recalled.", warning)
}

func (test *ConversationMessageServiceTest) TestDeleteWarningWithoutEmailIsEmpty() {
	Assert.Equals("", test.service().DeleteWarning(test.general()[0]))
}

func (test *ConversationMessageServiceTest) TestToggledAddsThenRemoves() {
	added := test.service().Toggled([]string{"m-1"}, "m-2")
	Assert.Equals([]string{"m-1", "m-2"}, added)
	Assert.Equals([]string{"m-1"}, test.service().Toggled(added, "m-2"))
}

func (test *ConversationMessageServiceTest) TestWithSenderAddsALowercaseAddressOnce() {
	senders := test.service().WithSender(nil, "example.com", " Carol@Outside.test ")
	senders = test.service().WithSender(senders, "example.com", "carol@outside.test")
	Assert.Equals([]string{"carol@outside.test"}, senders["example.com"])
	Assert.Equals(0, len(senders["example.org"]))
}

func (test *ConversationMessageServiceTest) TestReactionChoicesFollowMyReactions() {
	messages := test.general()
	messages[0].MyReactions = []string{"thumbs_up"}
	choices := test.service().ReactionChoices(messages, "m-3")
	Assert.Equals(12, len(choices))
	Assert.Equals("Remove thumbs up", choices[0].Label)
}

func (test *ConversationMessageServiceTest) TestHasReactionChecksMyReactions() {
	messages := test.general()
	messages[0].MyReactions = []string{"heart"}
	Assert.IsTrue(test.service().HasReaction(messages, "m-3", "heart"))
	Assert.IsTrue(!test.service().HasReaction(messages, "m-3", "thumbs_up"))
}

func (test *ConversationMessageServiceTest) TestAnnouncementNamesTheAuthor() {
	Assert.Equals("New message from Bo Lee", test.service().Announcement(test.general()[0], "ana@example.com"))
}

func (test *ConversationMessageServiceTest) TestMyOwnMessageIsNotAnnounced() {
	Assert.Equals("", test.service().Announcement(test.general()[1], "Ana@Example.com"))
}

func (test *ConversationMessageServiceTest) TestMissingMessageIsNotAnnounced() {
	Assert.Equals("", test.service().Announcement(Message{}, "ana@example.com"))
}
