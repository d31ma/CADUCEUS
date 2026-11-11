// @Test(ComposerTestSetup, {"partition":"tests"})
type ComposerServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ComposerServiceTest) composer() *ComposerService {
	return ComposerTestSetup{}.Composer(test.behaviors)
}

func (test *ComposerServiceTest) with(addresses string) *ComposerService {
	composer := test.composer()
	composer.AddTyped("to", addresses)
	return composer
}

func (test *ComposerServiceTest) sendWith(composer *ComposerService, body string) {
	composer.SendWith(body)
}

func (test *ComposerServiceTest) TestStartOpensANewConversationWithAToRow() {
	composer := test.composer()
	Assert.IsTrue(composer.NewConversation)
	Assert.IsTrue(composer.Rows[0].Shown)
	Assert.IsTrue(!composer.Rows[1].Shown)
	Assert.Equals("Name, channel or email address", composer.Rows[0].Placeholder)
}

func (test *ComposerServiceTest) TestOpenAnExistingConversationHidesTheRows() {
	composer := test.composer()
	composer.Open(SendTarget{ConversationId: "conv-1", Title: "#releases"})
	Assert.IsTrue(!composer.NewConversation)
	Assert.IsTrue(!composer.Rows[0].Shown)
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

func (test *ComposerServiceTest) TestDomainChangedStartsAnEmptyNewMessage() {
	composer := test.with("bo@example.com")
	composer.Open(SendTarget{ConversationId: "conv-1"})
	composer.Body = "draft"
	composer.DomainChanged(SessionDomainChanged{DomainId: "example.org"})
	Assert.IsTrue(composer.NewConversation)
	Assert.Equals("", composer.Body)
	Assert.Equals(0, len(composer.To))
}

func (test *ComposerServiceTest) TestAddTypedAddsChipsAndDoesNotSend() {
	composer := test.composer()
	composer.AddTyped("to", "bo@example.com carol@partner.org")
	Assert.Equals(2, len(composer.To))
	Assert.Equals("Add more people", composer.Rows[0].Placeholder)
	Assert.Equals([]string{"Bo Lee gets a chat message.", "carol@partner.org gets an email."}, composer.Summary)
	Assert.Equals("Send · 1 chat, 1 email", composer.SendLabel)
	Assert.IsTrue(composer.EmailMode)
}

func (test *ComposerServiceTest) TestAddTypedResolvesCcAndBlindCopyLikeTo() {
	composer := test.composer()
	composer.AddTyped("cc", "eve@example.org")
	composer.AddTyped("bcc", "carol@partner.org")
	Assert.Equals("otherDomain", composer.Cc[0].Kind)
	Assert.Equals("external", composer.BlindCopy[0].Kind)
	Assert.IsTrue(composer.Rows[1].Shown)
	Assert.IsTrue(composer.Rows[2].Shown)
}

func (test *ComposerServiceTest) TestAnotherDomainShowsTheEmailOnlyChip() {
	composer := test.with("eve@example.org")
	Assert.Equals("Email · other domain", composer.To[0].Label)
	Assert.Equals("Chat stays inside example.com", composer.To[0].ChatReason)
}

func (test *ComposerServiceTest) TestToggleCcShowsTheRow() {
	composer := test.composer()
	composer.ToggleCc()
	Assert.IsTrue(composer.Rows[1].Shown)
}

func (test *ComposerServiceTest) TestToggleBlindCopyShowsTheRow() {
	composer := test.composer()
	composer.ToggleBlindCopy()
	Assert.Equals("Hidden from everyone else", composer.Rows[2].Placeholder)
	Assert.IsTrue(composer.Rows[2].Shown)
}

func (test *ComposerServiceTest) TestKeepSubjectKeepsTheText() {
	composer := test.composer()
	composer.KeepSubject("Plan")
	Assert.Equals("Plan", composer.Subject)
}

func (test *ComposerServiceTest) TestSendWithTakesTheBody() {
	composer := test.with("bo@example.com")
	composer.SendWith("hello")
	Assert.Equals("Message sent.", composer.Announcement)
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
	test.sendWith(composer, "hello")
	Assert.Equals("addRecipient", composer.Problem)
	Assert.Equals("Add a recipient to send.", composer.ProblemText)
}

func (test *ComposerServiceTest) TestSendRefusesMoreThanTwentyRecipients() {
	composer := test.with("p0@partner.org p1@partner.org p2@partner.org p3@partner.org p4@partner.org p5@partner.org p6@partner.org p7@partner.org p8@partner.org p9@partner.org p10@partner.org p11@partner.org p12@partner.org p13@partner.org p14@partner.org p15@partner.org p16@partner.org p17@partner.org p18@partner.org p19@partner.org p20@partner.org")
	test.sendWith(composer, "hello")
	Assert.Equals("tooManyRecipients", composer.Problem)
}

func (test *ComposerServiceTest) TestSendRefusesAnInvalidAddress() {
	composer := test.with("carol@partner")
	test.sendWith(composer, "hello")
	Assert.Equals("fixRecipients", composer.Problem)
}

func (test *ComposerServiceTest) TestSendEmailWithoutSubjectMarksTheSubject() {
	composer := test.with("carol@partner.org")
	test.sendWith(composer, "hello")
	Assert.Equals("subjectMissing", composer.Problem)
	Assert.IsTrue(composer.SubjectMissing)
}

func (test *ComposerServiceTest) TestSendWithoutContentAsksForAMessage() {
	composer := test.with("bo@example.com")
	test.sendWith(composer, "   ")
	Assert.Equals("emptyMessage", composer.Problem)
}

func (test *ComposerServiceTest) TestSendWaitsForUploads() {
	composer := test.with("bo@example.com")
	composer.Picked(UploadPicked{FileName: "plan.pdf", Size: 10})
	test.sendWith(composer, "hello")
	Assert.Equals("waitForUploads", composer.Problem)
}

func (test *ComposerServiceTest) TestSendRefusesFailedUploads() {
	composer := test.with("bo@example.com")
	composer.Picked(UploadPicked{FileName: "broken.pdf", Size: 10})
	test.sendWith(composer, "hello")
	Assert.Equals("fixAttachments", composer.Problem)
}

func (test *ComposerServiceTest) TestTheRequestCarriesTheSubjectAndTheKindsTheUserSaw() {
	composer := test.with("bo@example.com carol@partner.org")
	composer.KeepSubject("Plan")
	composer.Body = "hello"
	request := composer.request(true)
	Assert.Equals("Plan", request.Email.Subject)
	Assert.Equals(2, len(request.Recipients))
	Assert.Equals("external", request.Recipients[1].ExpectedKind)
}

func (test *ComposerServiceTest) TestDeliverClearsTheComposerAfterSuccess() {
	composer := test.with("bo@example.com")
	test.sendWith(composer, "hello")
	Assert.Equals("", composer.Body)
	Assert.Equals(0, len(composer.To))
	Assert.Equals("conv-1", composer.Target.ConversationId)
	Assert.IsTrue(!composer.NewConversation)
	Assert.Equals("Message sent.", composer.Announcement)
}

func (test *ComposerServiceTest) TestDeliverKeepsTheMessageAndItsIdAfterAFailure() {
	composer := test.with("bo@example.com")
	test.sendWith(composer, "offline")
	Assert.Equals("unavailable", composer.Problem)
	Assert.Equals("offline", composer.Body)
	Assert.Equals("id-1", composer.send.PendingIds["conversation"])
}

func (test *ComposerServiceTest) TestARetryAfterAFailureSucceedsAndForgetsTheId() {
	composer := test.with("bo@example.com")
	test.sendWith(composer, "offline")
	BrowserFakes{}.Answer("secureRandom", "uuid", "id-2")
	test.sendWith(composer, "hello")
	Assert.Equals("", composer.Problem)
	Assert.Equals("", composer.send.PendingIds["conversation"])
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

func (test *ComposerServiceTest) TestTheSenderAddressIsShown() {
	Assert.Equals("ana@example.com", test.composer().SenderAddress)
}
