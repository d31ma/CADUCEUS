// @Test(ConversationTestSetup, {"partition":"tests"})
type ConversationDeliveryServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ConversationDeliveryServiceTest) lines(statuses ...DeliveryStatus) []DeliveryLine {
	return (&ConversationDeliveryService{}).Lines(statuses)
}

func (test *ConversationDeliveryServiceTest) summary(statuses ...DeliveryStatus) DeliveryLine {
	return (&ConversationDeliveryService{}).Summary(test.lines(statuses...))
}

func (test *ConversationDeliveryServiceTest) TestChatStatesHaveWordsAndTones() {
	lines := test.lines(
		DeliveryStatus{Recipient: "bo@example.com", Channel: "chat", Status: "delivered"},
		DeliveryStatus{Recipient: "bo@example.com", Channel: "chat", Status: "seen"},
		DeliveryStatus{Recipient: "bo@example.com", Channel: "chat", Status: "pending"},
	)
	Assert.Equals("Chat", lines[0].ChannelLabel)
	Assert.Equals("Delivered", lines[0].StatusLabel)
	Assert.Equals("ok", lines[0].Tone)
	Assert.Equals("Seen", lines[1].StatusLabel)
	Assert.Equals("ok", lines[1].Tone)
	Assert.Equals("Sending", lines[2].StatusLabel)
	Assert.Equals("info", lines[2].Tone)
}

func (test *ConversationDeliveryServiceTest) TestEmailStatesHaveWordsAndTones() {
	lines := test.lines(
		DeliveryStatus{Recipient: "carol@outside.test", Channel: "email", Status: "queued"},
		DeliveryStatus{Recipient: "carol@outside.test", Channel: "email", Status: "sent"},
		DeliveryStatus{Recipient: "carol@outside.test", Channel: "email", Status: "retrying"},
		DeliveryStatus{Recipient: "carol@outside.test", Channel: "email", Status: "deferred"},
		DeliveryStatus{Recipient: "carol@outside.test", Channel: "email", Status: "bounced"},
		DeliveryStatus{Recipient: "carol@outside.test", Channel: "email", Status: "failed"},
	)
	Assert.Equals("Email", lines[0].ChannelLabel)
	Assert.Equals("Queued", lines[0].StatusLabel)
	Assert.Equals("info", lines[0].Tone)
	Assert.Equals("Sent", lines[1].StatusLabel)
	Assert.Equals("ok", lines[1].Tone)
	Assert.Equals("Retrying", lines[2].StatusLabel)
	Assert.Equals("warning", lines[2].Tone)
	Assert.Equals("Retrying", lines[3].StatusLabel)
	Assert.Equals("Bounced", lines[4].StatusLabel)
	Assert.Equals("danger", lines[4].Tone)
	Assert.Equals("Failed", lines[5].StatusLabel)
	Assert.Equals("danger", lines[5].Tone)
}

func (test *ConversationDeliveryServiceTest) TestLineCopiesRecipientStatusAndDetail() {
	line := test.lines(DeliveryStatus{Recipient: "carol@outside.test", Channel: "email", Status: "bounced", Detail: "mailbox not found"})[0]
	Assert.Equals("carol@outside.test", line.Recipient)
	Assert.Equals("email", line.Channel)
	Assert.Equals("bounced", line.Status)
	Assert.Equals("mailbox not found", line.Detail)
}

func (test *ConversationDeliveryServiceTest) TestUnknownStateIsCalledUnknown() {
	line := test.lines(DeliveryStatus{Recipient: "bo@example.com", Channel: "pigeon", Status: "lost"})[0]
	Assert.Equals("Unknown", line.StatusLabel)
	Assert.Equals("info", line.Tone)
	Assert.Equals("", line.ChannelLabel)
}

func (test *ConversationDeliveryServiceTest) TestNoStatusesGiveNoLines() {
	Assert.Equals(0, len(test.lines()))
}

func (test *ConversationDeliveryServiceTest) TestSummaryNamesTheRecipientOfTheWorstProblem() {
	summary := test.summary(
		DeliveryStatus{Recipient: "bo@example.com", Channel: "chat", Status: "seen"},
		DeliveryStatus{Recipient: "dan@outside.test", Channel: "email", Status: "retrying"},
		DeliveryStatus{Recipient: "carol@outside.test", Channel: "email", Status: "bounced"},
	)
	Assert.Equals("Bounced for carol@outside.test", summary.StatusLabel)
	Assert.Equals("danger", summary.Tone)
	Assert.Equals("carol@outside.test", summary.Recipient)
}

func (test *ConversationDeliveryServiceTest) TestSummaryNamesARetryingRecipient() {
	summary := test.summary(
		DeliveryStatus{Recipient: "bo@example.com", Channel: "chat", Status: "delivered"},
		DeliveryStatus{Recipient: "dan@outside.test", Channel: "email", Status: "retrying"},
	)
	Assert.Equals("Retrying for dan@outside.test", summary.StatusLabel)
	Assert.Equals("warning", summary.Tone)
}

func (test *ConversationDeliveryServiceTest) TestSummaryCountsPeopleWhenNothingIsWrong() {
	summary := test.summary(
		DeliveryStatus{Recipient: "carol@outside.test", Channel: "email", Status: "sent"},
		DeliveryStatus{Recipient: "dan@outside.test", Channel: "email", Status: "sent"},
		DeliveryStatus{Recipient: "eve@outside.test", Channel: "email", Status: "sent"},
	)
	Assert.Equals("Sent to 3 people", summary.StatusLabel)
	Assert.Equals("ok", summary.Tone)
}

func (test *ConversationDeliveryServiceTest) TestSummaryShowsTheWaitingWord() {
	summary := test.summary(
		DeliveryStatus{Recipient: "bo@example.com", Channel: "chat", Status: "seen"},
		DeliveryStatus{Recipient: "carol@outside.test", Channel: "email", Status: "queued"},
	)
	Assert.Equals("Queued for 2 people", summary.StatusLabel)
	Assert.Equals("info", summary.Tone)
}

func (test *ConversationDeliveryServiceTest) TestSummaryOfOneLineIsItsWord() {
	Assert.Equals("Delivered", test.summary(DeliveryStatus{Recipient: "bo@example.com", Channel: "chat", Status: "delivered"}).StatusLabel)
}

func (test *ConversationDeliveryServiceTest) TestSummaryOfNoLinesIsEmpty() {
	summary := test.summary()
	Assert.Equals("", summary.StatusLabel)
	Assert.Equals("", summary.Tone)
}

func (test *ConversationDeliveryServiceTest) TestSummarySaysSeenByPeople() {
	summary := test.summary(
		DeliveryStatus{Recipient: "bo@example.com", Channel: "chat", Status: "seen"},
		DeliveryStatus{Recipient: "dee@example.com", Channel: "chat", Status: "seen"},
	)
	Assert.Equals("Seen by 2 people", summary.StatusLabel)
}
