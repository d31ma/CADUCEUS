// @Test(ComposerTestSetup, {"partition":"tests"})
type ComposerServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ComposerServiceTest) composer() *ComposerService {
	return ComposerTestSetup{}.Composer(test.behaviors)
}

func (test *ComposerServiceTest) with(addresses string) *ComposerService {
	composer := test.composer()
	composer.ToText = addresses
	composer.AddTyped("to")
	return composer
}

func (test *ComposerServiceTest) deliverPending(composer *ComposerService) {
	composer.Deliver(SendRequested{Request: composer.request(composer.emailMode())})
}

func (test *ComposerServiceTest) TestStartOpensANewConversationWithAToRow() {
	composer := test.composer()
	Assert.IsTrue(composer.NewConversation)
	Assert.Equals(1, len(composer.Rows))
	Assert.Equals("Name, channel or email address", composer.Rows[0].Placeholder)
}

func (test *ComposerServiceTest) TestOpenAnExistingConversationHidesTheRows() {
	composer := test.composer()
	composer.Open(SendTarget{ConversationId: "conv-1", Title: "#releases"})
	Assert.IsTrue(!composer.NewConversation)
	Assert.Equals(0, len(composer.Rows))
	Assert.IsTrue(!composer.EmailMode)
}

func (test *ComposerServiceTest) TestOpenAnEmailConversationPrefillsTheReplySubject() {
	composer := test.composer()
	composer.Open(SendTarget{ConversationId: "conv-1", Subject: "Contract renewal"})
	Assert.Equals("Re: Contract renewal", composer.Subject)
	Assert.IsTrue(composer.EmailMode)
}

func (test *ComposerServiceTest) TestOpenKeepsAnExistingReplyPrefix() {
	composer := test.composer()
	composer.Open(SendTarget{ConversationId: "conv-1", Subject: "RE: Contract"})
	Assert.Equals("RE: Contract", composer.Subject)
}

func (test *ComposerServiceTest) TestAddTypedShowsChipsAndSummary() {
	composer := test.with("bo@example.com carol@partner.org")
	Assert.Equals(2, len(composer.To))
	Assert.Equals("", composer.ToText)
	Assert.Equals("Add more people", composer.Rows[0].Placeholder)
	Assert.Equals([]string{"Bo Lee gets a chat message.", "carol@partner.org gets an email."}, composer.Summary)
	Assert.Equals("Send · 1 chat, 1 email", composer.SendLabel)
	Assert.IsTrue(composer.EmailMode)
}

func (test *ComposerServiceTest) TestRecipientKeyEnterAddsTheTypedAddress() {
	composer := test.composer()
	composer.RecipientKey("to", "Enter", "eve@example.org")
	Assert.Equals("Email · other domain", composer.To[0].Label)
	Assert.Equals("", composer.ToText)
}

func (test *ComposerServiceTest) TestRecipientKeyBackspaceInAnEmptyFieldRemovesTheLastChip() {
	composer := test.with("bo@example.com carol@partner.org")
	composer.RecipientKey("to", "Backspace", "")
	Assert.Equals(1, len(composer.To))
}

func (test *ComposerServiceTest) TestRecipientKeyOtherKeysOnlyKeepTheText() {
	composer := test.composer()
	composer.RecipientKey("cc", "a", "bo@exa")
	Assert.Equals("bo@exa", composer.CcText)
	Assert.Equals(0, len(composer.Cc))
}

func (test *ComposerServiceTest) TestTypeTextKeepsTheText() {
	composer := test.composer()
	composer.TypeText("bcc", "carol@")
	Assert.Equals("carol@", composer.BlindCopyText)
}

func (test *ComposerServiceTest) TestToggleCcAndBlindCopyShowTheirRows() {
	composer := test.composer()
	composer.ToggleCc()
	composer.ToggleBlindCopy()
	Assert.Equals(3, len(composer.Rows))
	Assert.Equals("Hidden from everyone else", composer.Rows[2].Placeholder)
}

func (test *ComposerServiceTest) TestCcRecipientsAreResolvedLikeTo() {
	composer := test.composer()
	composer.ToggleCc()
	composer.RecipientKey("cc", "Enter", "eve@example.org")
	Assert.Equals("otherDomain", composer.Cc[0].Kind)
}

func (test *ComposerServiceTest) TestChooseAnnouncesTheNewChoice() {
	composer := test.with("bo@example.com")
	composer.Choose("bo@example.com", "to", "email")
	Assert.Equals("Bo Lee now gets: Email.", composer.Announcement)
	Assert.Equals("Send email", composer.SendLabel)
}

func (test *ComposerServiceTest) TestRemoveTakesTheChipAway() {
	composer := test.with("bo@example.com")
	composer.Remove("bo@example.com", "to")
	Assert.Equals(0, len(composer.To))
}

func (test *ComposerServiceTest) TestSendWithoutRecipientsAsksForOne() {
	composer := test.composer()
	composer.Body = "hello"
	composer.Send()
	Assert.Equals("addRecipient", composer.Problem)
	Assert.Equals("Add a recipient to send.", composer.ProblemText)
	Assert.IsTrue(!composer.Sending)
}

func (test *ComposerServiceTest) TestSendRefusesMoreThanTwentyRecipients() {
	composer := test.with("p0@partner.org p1@partner.org p2@partner.org p3@partner.org p4@partner.org p5@partner.org p6@partner.org p7@partner.org p8@partner.org p9@partner.org p10@partner.org p11@partner.org p12@partner.org p13@partner.org p14@partner.org p15@partner.org p16@partner.org p17@partner.org p18@partner.org p19@partner.org p20@partner.org")
	composer.Body = "hello"
	composer.Send()
	Assert.Equals("tooManyRecipients", composer.Problem)
}

func (test *ComposerServiceTest) TestSendRefusesAnInvalidAddress() {
	composer := test.with("carol@partner")
	composer.Body = "hello"
	composer.Send()
	Assert.Equals("fixRecipients", composer.Problem)
}

func (test *ComposerServiceTest) TestSendEmailWithoutSubjectMarksTheSubject() {
	composer := test.with("carol@partner.org")
	composer.Body = "hello"
	composer.Send()
	Assert.Equals("subjectMissing", composer.Problem)
	Assert.IsTrue(composer.SubjectMissing)
}

func (test *ComposerServiceTest) TestSendWithoutContentAsksForAMessage() {
	composer := test.with("bo@example.com")
	composer.Body = "   "
	composer.Send()
	Assert.Equals("emptyMessage", composer.Problem)
}

func (test *ComposerServiceTest) TestSendWaitsForUploads() {
	composer := test.with("bo@example.com")
	composer.Body = "hello"
	composer.Picked(UploadPicked{FileName: "plan.pdf", Size: 10})
	composer.Send()
	Assert.Equals("waitForUploads", composer.Problem)
}

func (test *ComposerServiceTest) TestSendRefusesFailedUploads() {
	composer := test.with("bo@example.com")
	composer.Body = "hello"
	composer.Picked(UploadPicked{FileName: "broken.pdf", Size: 10})
	composer.Send()
	Assert.Equals("fixAttachments", composer.Problem)
}

func (test *ComposerServiceTest) TestSendWhileSendingWaits() {
	composer := test.with("bo@example.com")
	composer.Body = "hello"
	composer.Sending = true
	composer.Send()
	Assert.Equals("stillSending", composer.Problem)
}

func (test *ComposerServiceTest) TestSendStartsSendingAMixedMessage() {
	composer := test.with("bo@example.com carol@partner.org")
	composer.Subject = "Plan"
	composer.Body = "hello"
	composer.Send()
	Assert.Equals("", composer.Problem)
	Assert.IsTrue(composer.Sending)
	request := composer.request(true)
	Assert.Equals("Plan", request.Email.Subject)
	Assert.Equals(2, len(request.Recipients))
	Assert.Equals("external", request.Recipients[1].ExpectedKind)
}

func (test *ComposerServiceTest) TestDeliverClearsTheComposerAfterSuccess() {
	composer := test.with("bo@example.com")
	composer.Body = "hello"
	composer.Send()
	test.deliverPending(composer)
	Assert.IsTrue(!composer.Sending)
	Assert.Equals("", composer.Body)
	Assert.Equals(0, len(composer.To))
	Assert.Equals("conv-1", composer.Target.ConversationId)
	Assert.IsTrue(!composer.NewConversation)
	Assert.Equals("Message sent.", composer.Announcement)
}

func (test *ComposerServiceTest) TestDeliverKeepsTheMessageAndItsIdAfterAFailure() {
	composer := test.with("bo@example.com")
	composer.Body = "offline"
	composer.Send()
	test.deliverPending(composer)
	Assert.Equals("unavailable", composer.Problem)
	Assert.Equals("offline", composer.Body)
	Assert.Equals("id-1", composer.send.PendingId)
}

func (test *ComposerServiceTest) TestARetryAfterAFailureReusesTheClientMessageId() {
	composer := test.with("bo@example.com")
	composer.Body = "offline"
	test.deliverPending(composer)
	BrowserFakes{}.Answer("secureRandom", "uuid", "id-2")
	composer.Body = "hello"
	composer.Send()
	test.deliverPending(composer)
	Assert.Equals("", composer.Problem)
	Assert.Equals("", composer.send.PendingId)
}

func (test *ComposerServiceTest) TestRecipientsChangedOpensTheDialogWithTheChanges() {
	composer := test.with("eve@example.org")
	composer.recipients.Recipients[0] = composer.recipients.resolve.RecipientOf(ResolvedRecipient{Address: "eve@example.org", DisplayName: "Eve", Kind: "member", DefaultDelivery: "chat", AllowedDeliveries: []string{"chat", "email", "both"}}, "to", "chat")
	composer.Problem = composer.problems.OfFailure(&BehaviorError{Code: "HTTP_STATUS", Status: 409})
	composer.afterSend(SendResult{})
	composer.refresh()
	Assert.IsTrue(composer.ChangedOpen)
	Assert.Equals(1, len(composer.Changes))
	Assert.Equals("Email · other domain", composer.To[0].Label)
	Assert.Equals("Not sent. Some recipients changed. Check how each one gets it, then send again.", composer.ProblemText)
}

func (test *ComposerServiceTest) TestChatNotAllowedAlsoChecksTheRecipientsAgain() {
	composer := test.with("carol@partner.org")
	composer.Problem = composer.problems.OfFailure(&BehaviorError{Code: "HTTP_STATUS", Status: 422})
	composer.afterSend(SendResult{})
	Assert.IsTrue(composer.ChangedOpen)
	Assert.Equals(0, len(composer.Changes))
}

func (test *ComposerServiceTest) TestCloseChangedClosesTheDialog() {
	composer := test.composer()
	composer.ChangedOpen = true
	composer.CloseChanged()
	Assert.IsTrue(!composer.ChangedOpen)
}

func (test *ComposerServiceTest) TestBodyKeyControlEnterSends() {
	composer := test.with("bo@example.com")
	composer.Body = "hello"
	composer.BodyKey("Enter", true, false)
	Assert.IsTrue(composer.Sending)
}

func (test *ComposerServiceTest) TestBodyKeyPlainEnterOnlyAddsALine() {
	composer := test.with("bo@example.com")
	composer.Body = "hello"
	composer.BodyKey("Enter", false, false)
	Assert.IsTrue(!composer.Sending)
}

func (test *ComposerServiceTest) TestUploadsShowInTheComposer() {
	composer := test.composer()
	composer.Picked(UploadPicked{FileName: "plan.pdf", Size: 10})
	composer.Uploaded(UploadFinished{AttachmentId: "att-plan.pdf"})
	Assert.Equals("ready", composer.Uploads[0].State)
	composer.RetryUpload("plan.pdf")
	Assert.Equals(1, len(composer.Uploads))
	composer.RemoveUpload("plan.pdf")
	Assert.Equals(0, len(composer.Uploads))
}

func (test *ComposerServiceTest) TestRecipientKeyTabAddsAndOtherKeysDoNot() {
	composer := test.composer()
	composer.RecipientKey("to", "Tab", "bo@example.com")
	composer.RecipientKey("to", "Escape", "carol@partner.org")
	Assert.Equals(1, len(composer.To))
	Assert.Equals("carol@partner.org", composer.ToText)
}

func (test *ComposerServiceTest) TestBodyKeyCommandEnterSends() {
	composer := test.with("bo@example.com")
	composer.Body = "hello"
	composer.BodyKey("Enter", false, true)
	Assert.IsTrue(composer.Sending)
}

func (test *ComposerServiceTest) TestTheSenderAddressIsShown() {
	Assert.Equals("ana@example.com", test.composer().SenderAddress)
}
