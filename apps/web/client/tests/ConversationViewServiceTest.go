// @Test(ConversationTestSetup, {"partition":"tests"})
type ConversationViewServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ConversationViewServiceTest) service() *ConversationViewService {
	return &ConversationViewService{
		times:       &ConversationTimeService{},
		reactions:   &ConversationReactionService{},
		attachments: &ConversationAttachmentService{},
		deliveries:  &ConversationDeliveryService{},
	}
}

func (test *ConversationViewServiceTest) context() ConversationViewContext {
	return ConversationViewContext{MyAddress: "ana@example.com", DomainId: "example.com", Section: "direct", NowMs: 1794340800000}
}

func (test *ConversationViewServiceTest) views(context ConversationViewContext, newestFirst ...Message) []MessageView {
	return test.service().Views(newestFirst, context)
}

func (test *ConversationViewServiceTest) one(message Message) MessageView {
	return test.views(test.context(), message)[0]
}

func (test *ConversationViewServiceTest) message(messageId string, address string, createdAt string) Message {
	return Message{
		MessageId:      messageId,
		ConversationId: "c1",
		Author:         Author{Kind: "member", Address: address},
		Blocks:         []Block{test.paragraph("Hello")},
		CreatedAt:      createdAt,
	}
}

func (test *ConversationViewServiceTest) mine(messageId string, createdAt string) Message {
	message := test.message(messageId, "ana@example.com", createdAt)
	message.Author.DisplayName = "Ana Ruiz"
	return message
}

func (test *ConversationViewServiceTest) fromCarol(messageId string, createdAt string, blocks ...Block) Message {
	return Message{
		MessageId: messageId,
		Author:    Author{Kind: "external", Address: "carol@partner.org", DisplayName: "Carol Diaz"},
		Blocks:    blocks,
		CreatedAt: createdAt,
		Email: &MessageEmailSummary{
			Subject:     "Re: Contract renewal 2027",
			From:        "Carol Diaz <Carol@Partner.org>",
			To:          []string{"ana@example.com", "bo@example.com"},
			Cc:          []string{"legal@partner.org"},
			AuthResults: MessageAuthResults{SenderPolicy: "pass", SignatureCheck: "pass", DomainPolicy: "pass"},
		},
	}
}

func (test *ConversationViewServiceTest) handoff() Message {
	message := test.withResults(test.fromCarol("m1", "2026-11-10T08:47:00Z"), false, MessageAuthResults{Handoff: true})
	message.Author = Author{Kind: "otherDomain", Address: "Lee@Example.org", DisplayName: "Lee Chan"}
	message.Email.From = "Lee Chan <lee@example.org>"
	return message
}

func (test *ConversationViewServiceTest) withResults(message Message, suspicious bool, results MessageAuthResults) Message {
	message.Email.Suspicious = suspicious
	message.Email.AuthResults = results
	return message
}

func (test *ConversationViewServiceTest) paragraph(text string) Block {
	return Block{Kind: "paragraph", Spans: []Span{{Text: text}}}
}

func (test *ConversationViewServiceTest) quote(text string) Block {
	return Block{Kind: "quote", Spans: []Span{{Text: text}}}
}

func (test *ConversationViewServiceTest) remoteImage(url string) Block {
	return Block{Kind: "image", Image: &ImageRef{Alt: "logo", RemoteUrl: url}}
}

func (test *ConversationViewServiceTest) attachedImage() Block {
	return Block{Kind: "image", Image: &ImageRef{Alt: "chart", AttachmentId: "a9"}}
}

func (test *ConversationViewServiceTest) TestNoMessagesGiveNoViews() {
	Assert.Equals(0, len(test.views(test.context())))
}

func (test *ConversationViewServiceTest) TestViewsAreOldestFirst() {
	views := test.views(test.context(), test.mine("m2", "2026-11-10T14:05:00Z"), test.mine("m1", "2026-11-10T14:00:00Z"))
	Assert.Equals("m1", views[0].MessageId)
	Assert.Equals("m2", views[1].MessageId)
}

func (test *ConversationViewServiceTest) TestAuthorUsesTheDisplayName() {
	view := test.one(test.mine("m1", "2026-11-10T14:05:00Z"))
	Assert.Equals("Ana Ruiz", view.AuthorName)
	Assert.Equals("AR", view.AuthorInitials)
	Assert.Equals("ana@example.com", view.AuthorAddress)
	Assert.IsTrue(!view.Outside)
}

func (test *ConversationViewServiceTest) TestAuthorWithoutANameUsesTheAddress() {
	view := test.one(test.message("m1", "carol@partner.org", "2026-11-10T14:05:00Z"))
	Assert.Equals("carol@partner.org", view.AuthorName)
	Assert.Equals("CA", view.AuthorInitials)
}

func (test *ConversationViewServiceTest) TestOneWordNameGivesTwoLetters() {
	message := test.message("m1", "bo@example.com", "2026-11-10T14:05:00Z")
	message.Author.DisplayName = "bo"
	Assert.Equals("BO", test.one(message).AuthorInitials)
}

func (test *ConversationViewServiceTest) TestAuthorFromOutsideIsMarked() {
	Assert.IsTrue(test.one(test.fromCarol("m1", "2026-11-10T14:05:00Z")).Outside)
}

func (test *ConversationViewServiceTest) TestMineIgnoresLetterCase() {
	context := test.context()
	context.MyAddress = "Ana@Example.COM"
	views := test.views(context, test.message("m2", "bo@example.com", "2026-11-10T14:06:00Z"), test.mine("m1", "2026-11-10T14:05:00Z"))
	Assert.IsTrue(views[0].Mine)
	Assert.IsTrue(!views[1].Mine)
}

func (test *ConversationViewServiceTest) TestTimeAndDayComeFromTheCreationTime() {
	view := test.one(test.mine("m1", "2026-11-10T14:05:00Z"))
	Assert.Equals("14:05", view.Time)
	Assert.Equals("2026-11-10T14:05:00Z", view.DateTime)
	Assert.Equals("Today", view.DayLabel)
}

func (test *ConversationViewServiceTest) newYork() ConversationViewContext {
	context := test.context()
	context.TimeZone = "America/New_York"
	return context
}

func (test *ConversationViewServiceTest) TestTimeAndDayUseTheLocalTimeZone() {
	views := test.views(test.newYork(), test.mine("m2", "2026-11-10T14:05:00Z"), test.mine("m1", "2026-11-10T03:00:00Z"))
	Assert.Equals("22:00", views[0].Time)
	Assert.Equals("Yesterday", views[0].DayLabel)
	Assert.Equals("09:05", views[1].Time)
	Assert.Equals("Today", views[1].DayLabel)
	Assert.Equals("Actions for the message from Ana Ruiz at 09:05", views[1].ActionsLabel)
}

func (test *ConversationViewServiceTest) TestLocalMidnightStartsANewDayAndBreaksTheGroup() {
	views := test.views(test.newYork(), test.mine("m2", "2026-11-10T05:01:00Z"), test.mine("m1", "2026-11-10T04:58:00Z"))
	Assert.Equals("Yesterday", views[0].DayLabel)
	Assert.Equals("Today", views[1].DayLabel)
	Assert.IsTrue(views[1].StartsDay)
	Assert.IsTrue(!views[1].Grouped)
}

func (test *ConversationViewServiceTest) TestUniversalMidnightDoesNotBreakTheGroupInTheLocalTimeZone() {
	views := test.views(test.newYork(), test.mine("m2", "2026-11-11T00:02:00Z"), test.mine("m1", "2026-11-10T23:58:00Z"))
	Assert.Equals("Today", views[1].DayLabel)
	Assert.IsTrue(!views[1].StartsDay)
	Assert.IsTrue(views[1].Grouped)
}

func (test *ConversationViewServiceTest) TestFirstMessageOfEachDayStartsTheDay() {
	views := test.views(test.context(),
		test.mine("m3", "2026-11-10T09:01:00Z"),
		test.mine("m2", "2026-11-10T09:00:00Z"),
		test.mine("m1", "2026-11-09T09:00:00Z"),
	)
	Assert.IsTrue(views[0].StartsDay)
	Assert.Equals("Yesterday", views[0].DayLabel)
	Assert.IsTrue(views[1].StartsDay)
	Assert.IsTrue(!views[2].StartsDay)
}

func (test *ConversationViewServiceTest) TestOneAuthorWithinFiveMinutesIsGrouped() {
	views := test.views(test.context(), test.mine("m2", "2026-11-10T14:05:00Z"), test.mine("m1", "2026-11-10T14:00:00Z"))
	Assert.IsTrue(!views[0].Grouped)
	Assert.IsTrue(views[1].Grouped)
}

func (test *ConversationViewServiceTest) TestMoreThanFiveMinutesApartIsNotGrouped() {
	views := test.views(test.context(), test.mine("m2", "2026-11-10T14:05:01Z"), test.mine("m1", "2026-11-10T14:00:00Z"))
	Assert.IsTrue(!views[1].Grouped)
}

func (test *ConversationViewServiceTest) TestDifferentAuthorsAreNotGrouped() {
	views := test.views(test.context(), test.message("m2", "bo@example.com", "2026-11-10T14:01:00Z"), test.mine("m1", "2026-11-10T14:00:00Z"))
	Assert.IsTrue(!views[1].Grouped)
}

func (test *ConversationViewServiceTest) TestEmailMessagesAreNotGrouped() {
	views := test.views(test.context(),
		test.fromCarol("m3", "2026-11-10T14:02:00Z"),
		test.fromCarol("m2", "2026-11-10T14:01:00Z"),
		test.message("m1", "carol@partner.org", "2026-11-10T14:00:00Z"),
	)
	Assert.IsTrue(!views[1].Grouped)
	Assert.IsTrue(!views[2].Grouped)
}

func (test *ConversationViewServiceTest) TestMessageAfterADeletedOneIsNotGrouped() {
	deleted := test.mine("m1", "2026-11-10T14:00:00Z")
	deleted.DeletedAt = "2026-11-10T14:03:00Z"
	views := test.views(test.context(), test.mine("m2", "2026-11-10T14:01:00Z"), deleted)
	Assert.IsTrue(!views[1].Grouped)
}

func (test *ConversationViewServiceTest) TestMessagesOnDifferentDaysAreNotGrouped() {
	views := test.views(test.context(), test.mine("m2", "2026-11-10T00:01:00Z"), test.mine("m1", "2026-11-09T23:58:00Z"))
	Assert.IsTrue(views[1].StartsDay)
	Assert.IsTrue(!views[1].Grouped)
}

func (test *ConversationViewServiceTest) TestNewDividerIsOnTheFirstUnreadMessage() {
	context := test.context()
	context.UnreadCount = 2
	views := test.views(context,
		test.message("m3", "bo@example.com", "2026-11-10T14:03:00Z"),
		test.message("m2", "bo@example.com", "2026-11-10T14:02:00Z"),
		test.mine("m1", "2026-11-10T14:01:00Z"),
	)
	Assert.IsTrue(!views[0].NewDivider)
	Assert.IsTrue(views[1].NewDivider)
	Assert.IsTrue(!views[2].NewDivider)
}

func (test *ConversationViewServiceTest) TestNoNewDividerWithoutUnreadMessages() {
	views := test.views(test.context(), test.mine("m2", "2026-11-10T14:02:00Z"), test.mine("m1", "2026-11-10T14:01:00Z"))
	Assert.IsTrue(!views[0].NewDivider)
	Assert.IsTrue(!views[1].NewDivider)
}

func (test *ConversationViewServiceTest) TestNoNewDividerWhenMoreAreUnreadThanLoaded() {
	context := test.context()
	context.UnreadCount = 5
	views := test.views(context, test.mine("m2", "2026-11-10T14:02:00Z"), test.mine("m1", "2026-11-10T14:01:00Z"))
	Assert.IsTrue(!views[0].NewDivider)
	Assert.IsTrue(!views[1].NewDivider)
}

func (test *ConversationViewServiceTest) TestEditedMessageIsMarked() {
	message := test.mine("m1", "2026-11-10T14:00:00Z")
	message.EditedAt = "2026-11-10T14:10:00Z"
	view := test.one(message)
	Assert.IsTrue(view.Edited)
	Assert.IsTrue(!view.Deleted)
}

func (test *ConversationViewServiceTest) TestDeletedMessageHidesItsContent() {
	message := test.mine("m1", "2026-11-10T14:00:00Z")
	message.DeletedAt = "2026-11-10T14:10:00Z"
	message.Reactions = map[string]int{"thumbs_up": 1}
	message.Attachments = []AttachmentRef{{AttachmentId: "a1", FileName: "price-list.pdf", MediaType: "application/pdf", Size: 100}}
	view := test.one(message)
	Assert.IsTrue(view.Deleted)
	Assert.Equals(0, len(view.Blocks))
	Assert.Equals(0, len(view.QuotedBlocks))
	Assert.Equals(0, len(view.Reactions))
	Assert.Equals(0, len(view.Attachments))
	Assert.IsTrue(!view.CanEdit)
	Assert.IsTrue(!view.CanDelete)
}

func (test *ConversationViewServiceTest) TestMentionOfMeIsFound() {
	message := test.message("m1", "bo@example.com", "2026-11-10T14:00:00Z")
	message.Blocks = []Block{test.paragraph("Hi "), {Kind: "paragraph", Spans: []Span{{Text: "ask "}, {Text: "@ANA", Bold: true}}}}
	Assert.IsTrue(test.one(message).MentionsMe)
}

func (test *ConversationViewServiceTest) TestMentionOfSomeoneElseIsNotMine() {
	message := test.message("m1", "bo@example.com", "2026-11-10T14:00:00Z")
	message.Blocks = []Block{test.paragraph("ask @chen")}
	Assert.IsTrue(!test.one(message).MentionsMe)
}

func (test *ConversationViewServiceTest) TestNoAddressMeansNoMention() {
	context := test.context()
	context.MyAddress = ""
	message := test.message("m1", "bo@example.com", "2026-11-10T14:00:00Z")
	message.Blocks = []Block{test.paragraph("write to x@example.com")}
	Assert.IsTrue(!test.views(context, message)[0].MentionsMe)
}

func (test *ConversationViewServiceTest) TestChatNoteInAnEmailConversationIsInternal() {
	context := test.context()
	context.Section = "email"
	views := test.views(context, test.fromCarol("m2", "2026-11-10T14:05:00Z"), test.message("m1", "bo@example.com", "2026-11-10T14:00:00Z"))
	Assert.IsTrue(views[0].InternalOnly)
	Assert.IsTrue(!views[1].InternalOnly)
}

func (test *ConversationViewServiceTest) TestChatOutsideAnEmailConversationIsNotInternal() {
	Assert.IsTrue(!test.one(test.mine("m1", "2026-11-10T14:00:00Z")).InternalOnly)
}

func (test *ConversationViewServiceTest) TestEmailCardShowsTheHeaders() {
	email := test.one(test.fromCarol("m1", "2026-11-10T08:47:00Z", test.paragraph("Hi Ana"))).Email
	Assert.IsTrue(email.IsEmail)
	Assert.Equals("Re: Contract renewal 2027", email.Subject)
	Assert.Equals("Carol Diaz <Carol@Partner.org>", email.From)
	Assert.Equals("ana@example.com, bo@example.com", email.To)
	Assert.Equals("legal@partner.org", email.Cc)
}

func (test *ConversationViewServiceTest) TestPassingDmarcIsAVerifiedSender() {
	email := test.one(test.fromCarol("m1", "2026-11-10T08:47:00Z")).Email
	Assert.Equals("verified", email.SenderCheck)
	Assert.Equals("Verified sender", email.SenderCheckLabel)
	Assert.Equals("SPF pass · DKIM pass · DMARC pass", email.SenderCheckInfo)
}

func (test *ConversationViewServiceTest) TestSuspiciousEmailIsAWarning() {
	message := test.withResults(test.fromCarol("m1", "2026-11-10T08:47:00Z"), true, MessageAuthResults{SenderPolicy: "pass", SignatureCheck: "pass", DomainPolicy: "pass"})
	email := test.one(message).Email
	Assert.Equals("warning", email.SenderCheck)
	Assert.Equals("Could not verify the sender", email.SenderCheckLabel)
}

func (test *ConversationViewServiceTest) TestFailingChecksAreAWarningWithTheirResults() {
	message := test.withResults(test.fromCarol("m1", "2026-11-10T08:47:00Z"), false, MessageAuthResults{SenderPolicy: "softfail", DomainPolicy: "fail"})
	email := test.one(message).Email
	Assert.Equals("warning", email.SenderCheck)
	Assert.Equals("SPF softfail · DKIM none · DMARC fail", email.SenderCheckInfo)
}

func (test *ConversationViewServiceTest) TestEmailFromAnotherDomainOnThisServerIsAHandoff() {
	message := test.handoff()
	email := test.one(message).Email
	Assert.Equals("handoff", email.SenderCheck)
	Assert.Equals("Sent on this server", email.SenderCheckLabel)
	Assert.Equals("Sent from example.org on this server", email.SenderCheckInfo)
}

func (test *ConversationViewServiceTest) TestPassingDmarcStillShowsAHandoff() {
	message := test.handoff()
	message.Email.AuthResults = MessageAuthResults{DomainPolicy: "pass", Handoff: true}
	Assert.Equals("handoff", test.one(message).Email.SenderCheck)
}

func (test *ConversationViewServiceTest) TestAnotherDomainWithoutTheHandoffFlagIsAWarning() {
	message := test.handoff()
	message.Email.AuthResults = MessageAuthResults{}
	Assert.Equals("warning", test.one(message).Email.SenderCheck)
}

func (test *ConversationViewServiceTest) TestChatFromAnotherDomainHasNoEmailCard() {
	message := test.handoff()
	message.Email = nil
	email := test.one(message).Email
	Assert.IsTrue(!email.IsEmail)
	Assert.Equals("", email.SenderCheck)
}

func (test *ConversationViewServiceTest) TestExternalEmailWithoutResultsIsAWarning() {
	message := test.withResults(test.fromCarol("m1", "2026-11-10T08:47:00Z"), false, MessageAuthResults{})
	email := test.one(message).Email
	Assert.Equals("warning", email.SenderCheck)
	Assert.Equals("SPF none · DKIM none · DMARC none", email.SenderCheckInfo)
}

func (test *ConversationViewServiceTest) TestRemoteImagesFromAnUnknownSenderAreHidden() {
	message := test.fromCarol("m1", "2026-11-10T08:47:00Z", test.remoteImage("https://partner.org/a.png"), test.attachedImage(), test.remoteImage("https://partner.org/b.png"))
	email := test.one(message).Email
	Assert.Equals(2, email.BlockedImages)
	Assert.Equals("2 images hidden", email.BlockedLabel)
	Assert.IsTrue(!email.ImagesAllowed)
}

func (test *ConversationViewServiceTest) TestOneHiddenImageIsSingular() {
	message := test.fromCarol("m1", "2026-11-10T08:47:00Z", test.paragraph("Logo:"), test.remoteImage("https://partner.org/a.png"))
	Assert.Equals("1 image hidden", test.one(message).Email.BlockedLabel)
}

func (test *ConversationViewServiceTest) TestAllowedSenderShowsImages() {
	context := test.context()
	context.AllowedSenders = []string{"carol@partner.org"}
	message := test.fromCarol("m1", "2026-11-10T08:47:00Z", test.remoteImage("https://partner.org/a.png"))
	email := test.views(context, message)[0].Email
	Assert.Equals(0, email.BlockedImages)
	Assert.Equals("", email.BlockedLabel)
	Assert.IsTrue(email.ImagesAllowed)
}

func (test *ConversationViewServiceTest) TestChatMessageHasNoEmailCard() {
	email := test.one(test.mine("m1", "2026-11-10T14:00:00Z")).Email
	Assert.IsTrue(!email.IsEmail)
	Assert.Equals("", email.SenderCheck)
	Assert.Equals("", email.SenderCheckLabel)
}

func (test *ConversationViewServiceTest) TestTrailingQuotesOfAnEmailAreSplitOff() {
	message := test.fromCarol("m1", "2026-11-10T08:47:00Z", test.paragraph("Hi"), test.quote("inline"), test.paragraph("Thanks"), test.quote("earlier"), test.quote("older"))
	view := test.one(message)
	Assert.Equals(3, len(view.Blocks))
	Assert.Equals(2, len(view.QuotedBlocks))
	Assert.Equals("earlier", view.QuotedBlocks[0].Spans[0].Text)
	Assert.IsTrue(view.Email.HasQuoted)
	Assert.IsTrue(!view.ShowQuoted)
}

func (test *ConversationViewServiceTest) TestShownQuotesStayInTheQuotedBlocks() {
	context := test.context()
	context.ShowQuoted = []string{"m1"}
	view := test.views(context, test.fromCarol("m1", "2026-11-10T08:47:00Z", test.paragraph("Hi"), test.quote("earlier")))[0]
	Assert.IsTrue(view.ShowQuoted)
	Assert.Equals(1, len(view.Blocks))
	Assert.Equals(1, len(view.QuotedBlocks))
}

func (test *ConversationViewServiceTest) TestEmailWithoutQuotesHasNoQuotedText() {
	view := test.one(test.fromCarol("m1", "2026-11-10T08:47:00Z", test.paragraph("Hi")))
	Assert.Equals(1, len(view.Blocks))
	Assert.Equals(0, len(view.QuotedBlocks))
	Assert.IsTrue(!view.Email.HasQuoted)
}

func (test *ConversationViewServiceTest) TestChatQuotesStayInTheBody() {
	message := test.mine("m1", "2026-11-10T14:00:00Z")
	message.Blocks = []Block{test.paragraph("See"), test.quote("this")}
	view := test.one(message)
	Assert.Equals(2, len(view.Blocks))
	Assert.Equals(0, len(view.QuotedBlocks))
}

func (test *ConversationViewServiceTest) TestTruncatedIsCopied() {
	message := test.fromCarol("m1", "2026-11-10T08:47:00Z", test.paragraph("Long"))
	message.Truncated = true
	Assert.IsTrue(test.one(message).Truncated)
}

func (test *ConversationViewServiceTest) TestReactionsAndAttachmentsAreShown() {
	message := test.message("m1", "bo@example.com", "2026-11-10T14:00:00Z")
	message.Reactions = map[string]int{"thumbs_up": 2}
	message.MyReactions = []string{"thumbs_up"}
	message.Attachments = []AttachmentRef{{AttachmentId: "a1", FileName: "renewal-2027-draft.pdf", MediaType: "application/pdf", Size: 217088}}
	view := test.one(message)
	Assert.Equals("thumbs up: 2, including you", view.Reactions[0].Label)
	Assert.Equals("212 KB", view.Attachments[0].SizeLabel)
	Assert.Equals("PDF", view.Attachments[0].TypeLabel)
}

func (test *ConversationViewServiceTest) TestMyMessageShowsItsDelivery() {
	message := test.mine("m1", "2026-11-10T14:00:00Z")
	message.Deliveries = []DeliveryStatus{
		{Recipient: "bo@example.com", Channel: "chat", Status: "read"},
		{Recipient: "carol@outside.test", Channel: "email", Status: "bounced", Detail: "mailbox not found"},
	}
	view := test.one(message)
	Assert.IsTrue(view.ShowDelivery)
	Assert.Equals(2, len(view.Deliveries))
	Assert.Equals("Bounced for carol@outside.test", view.DeliverySummary.StatusLabel)
}

func (test *ConversationViewServiceTest) TestMyMessageWithoutDeliveriesShowsNone() {
	Assert.IsTrue(!test.one(test.mine("m1", "2026-11-10T14:00:00Z")).ShowDelivery)
}

func (test *ConversationViewServiceTest) TestSomeoneElsesDeliveryIsNotShown() {
	message := test.message("m1", "bo@example.com", "2026-11-10T14:00:00Z")
	message.Deliveries = []DeliveryStatus{{Recipient: "ana@example.com", Channel: "chat", Status: "read"}}
	Assert.IsTrue(!test.one(message).ShowDelivery)
}

func (test *ConversationViewServiceTest) TestReplyLabels() {
	none := test.mine("m3", "2026-11-10T14:03:00Z")
	single := test.mine("m2", "2026-11-10T14:02:00Z")
	single.ReplyCount = 1
	several := test.mine("m1", "2026-11-10T14:01:00Z")
	several.ReplyCount = 4
	views := test.views(test.context(), none, single, several)
	Assert.Equals("4 replies", views[0].ReplyLabel)
	Assert.Equals("1 reply", views[1].ReplyLabel)
	Assert.Equals("", views[2].ReplyLabel)
}

func (test *ConversationViewServiceTest) TestMyChatMessageCanBeEditedAndDeleted() {
	view := test.one(test.mine("m1", "2026-11-10T14:00:00Z"))
	Assert.IsTrue(view.CanEdit)
	Assert.IsTrue(view.CanDelete)
}

func (test *ConversationViewServiceTest) TestSomeoneElsesMessageCannotBeChanged() {
	view := test.one(test.message("m1", "bo@example.com", "2026-11-10T14:00:00Z"))
	Assert.IsTrue(!view.CanEdit)
	Assert.IsTrue(!view.CanDelete)
}

func (test *ConversationViewServiceTest) TestMyEmailCanBeDeletedButNotEdited() {
	message := test.fromCarol("m1", "2026-11-10T14:00:00Z", test.paragraph("Hi"))
	message.Author = Author{Kind: "member", Address: "ana@example.com", DisplayName: "Ana Ruiz"}
	view := test.one(message)
	Assert.IsTrue(!view.CanEdit)
	Assert.IsTrue(view.CanDelete)
}

func (test *ConversationViewServiceTest) TestEditingMarksTheMessage() {
	context := test.context()
	context.EditingId = "m2"
	views := test.views(context, test.mine("m2", "2026-11-10T14:02:00Z"), test.mine("m1", "2026-11-10T14:01:00Z"))
	Assert.IsTrue(!views[0].Editing)
	Assert.IsTrue(views[1].Editing)
}

func (test *ConversationViewServiceTest) TestActionsLabelNamesTheAuthorAndTime() {
	Assert.Equals("Actions for the message from Ana Ruiz at 14:05", test.one(test.mine("m1", "2026-11-10T14:05:00Z")).ActionsLabel)
}
