// @Test(ComposerTestSetup, {"partition":"tests"})
type ComposerWordsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ComposerWordsServiceTest) words() *ComposerWordsService {
	return &ComposerWordsService{}
}

func (test *ComposerWordsServiceTest) ready(name string, kind string, delivery string) Recipient {
	return Recipient{Address: "x@example.com", DisplayName: name, Kind: kind, Delivery: delivery, State: "ready"}
}

func (test *ComposerWordsServiceTest) TestChipLabelShowsChatForAMember() {
	Assert.Equals("Chat", test.words().ChipLabel(test.ready("Bo Lee", "member", "chat")))
}

func (test *ComposerWordsServiceTest) TestChipLabelShowsChatForAChannel() {
	Assert.Equals("Chat", test.words().ChipLabel(test.ready("releases", "channel", "chat")))
}

func (test *ComposerWordsServiceTest) TestChipLabelShowsEmailForAMemberSwitchedToEmail() {
	Assert.Equals("Email", test.words().ChipLabel(test.ready("Bo Lee", "member", "email")))
}

func (test *ComposerWordsServiceTest) TestChipLabelShowsBoth() {
	Assert.Equals("Chat + email", test.words().ChipLabel(test.ready("Bo Lee", "member", "both")))
}

func (test *ComposerWordsServiceTest) TestChipLabelMarksAnotherHostedDomain() {
	Assert.Equals("Email · other domain", test.words().ChipLabel(test.ready("eve@example.org", "otherDomain", "email")))
}

func (test *ComposerWordsServiceTest) TestChipLabelShowsEmailForAnOutsideAddress() {
	Assert.Equals("Email", test.words().ChipLabel(test.ready("carol@partner.org", "external", "email")))
}

func (test *ComposerWordsServiceTest) TestChipLabelShowsChecking() {
	Assert.Equals("Checking…", test.words().ChipLabel(Recipient{State: "checking"}))
}

func (test *ComposerWordsServiceTest) TestChipLabelShowsNotValid() {
	Assert.Equals("Not valid", test.words().ChipLabel(Recipient{State: "invalid"}))
}

func (test *ComposerWordsServiceTest) TestChipAriaLabelNamesTheChoice() {
	recipient := test.ready("Bo Lee", "member", "chat")
	recipient.Address = "bo@example.com"
	Assert.Equals("Bo Lee, Chat, bo@example.com. Open menu", test.words().ChipAriaLabel(recipient))
}

func (test *ComposerWordsServiceTest) TestChatReasonForAnotherDomainNamesTheSenderDomain() {
	Assert.Equals("Chat stays inside example.com", test.words().ChatReason("otherDomain", "eve@example.org", "example.com"))
}

func (test *ComposerWordsServiceTest) TestChatReasonForAnOutsideAddressNamesTheAddress() {
	Assert.Equals("carol@partner.org has no Mesail account", test.words().ChatReason("external", "carol@partner.org", "example.com"))
}

func (test *ComposerWordsServiceTest) TestChatReasonIsEmptyForAMember() {
	Assert.Equals("", test.words().ChatReason("member", "bo@example.com", "example.com"))
}

func (test *ComposerWordsServiceTest) TestBothReasonForAnotherDomain() {
	Assert.Equals("Not possible: this person is in another domain", test.words().BothReason("otherDomain"))
}

func (test *ComposerWordsServiceTest) TestBothReasonForAnOutsideAddress() {
	Assert.Equals("Not possible without a Mesail account", test.words().BothReason("external"))
}

func (test *ComposerWordsServiceTest) TestNamesJoinsNobody() {
	Assert.Equals("", test.words().Names([]string{}))
}

func (test *ComposerWordsServiceTest) TestNamesJoinsOne() {
	Assert.Equals("Bo", test.words().Names([]string{"Bo"}))
}

func (test *ComposerWordsServiceTest) TestNamesJoinsTwoWithAnd() {
	Assert.Equals("Bo and Carol", test.words().Names([]string{"Bo", "Carol"}))
}

func (test *ComposerWordsServiceTest) TestNamesJoinsThreeWithoutASerialComma() {
	Assert.Equals("Bo, Carol and Finn", test.words().Names([]string{"Bo", "Carol", "Finn"}))
}

func (test *ComposerWordsServiceTest) TestSummarySaysWhoGetsWhat() {
	summary := test.words().Summary([]Recipient{
		test.ready("Bo Lee", "member", "chat"),
		test.ready("Carol Diaz", "external", "email"),
		test.ready("Finn Moss", "otherDomain", "email"),
	})
	Assert.Equals([]string{"Bo Lee gets a chat message.", "Carol Diaz and Finn Moss get an email."}, summary)
}

func (test *ComposerWordsServiceTest) TestSummaryNamesBoth() {
	summary := test.words().Summary([]Recipient{test.ready("Bo Lee", "member", "both")})
	Assert.Equals([]string{"Bo Lee gets a chat message and an email."}, summary)
}

func (test *ComposerWordsServiceTest) TestSummaryLeavesOutRecipientsStillChecking() {
	summary := test.words().Summary([]Recipient{{DisplayName: "nora@clientco.net", State: "checking"}})
	Assert.Equals(0, len(summary))
}

func (test *ComposerWordsServiceTest) TestSendLabelForChatOnly() {
	Assert.Equals("Send", test.words().SendLabel([]Recipient{test.ready("Bo Lee", "member", "chat")}))
}

func (test *ComposerWordsServiceTest) TestSendLabelWithoutRecipients() {
	Assert.Equals("Send", test.words().SendLabel([]Recipient{}))
}

func (test *ComposerWordsServiceTest) TestSendLabelForEmailOnly() {
	Assert.Equals("Send email", test.words().SendLabel([]Recipient{test.ready("Carol Diaz", "external", "email")}))
}

func (test *ComposerWordsServiceTest) TestSendLabelCountsBothInEachNumber() {
	label := test.words().SendLabel([]Recipient{
		test.ready("Bo Lee", "member", "both"),
		test.ready("Carol Diaz", "external", "email"),
		test.ready("Finn Moss", "otherDomain", "email"),
	})
	Assert.Equals("Send · 1 chat, 3 emails", label)
}

func (test *ComposerWordsServiceTest) TestEmailModeIsOnWithAnEmailRecipient() {
	Assert.IsTrue(test.words().EmailMode([]Recipient{test.ready("Carol Diaz", "external", "email")}))
}

func (test *ComposerWordsServiceTest) TestEmailModeIsOffForChatOnly() {
	Assert.IsTrue(!test.words().EmailMode([]Recipient{test.ready("Bo Lee", "member", "chat")}))
}

func (test *ComposerWordsServiceTest) TestModeTextIsEmptyForChat() {
	Assert.Equals("", test.words().ModeText([]Recipient{test.ready("Bo Lee", "member", "chat")}, "example.com"))
}

func (test *ComposerWordsServiceTest) TestModeTextNamesTheOutsidePerson() {
	text := test.words().ModeText([]Recipient{test.ready("Carol Diaz", "external", "email")}, "example.com")
	Assert.Equals("This is an email. Carol Diaz is outside example.com.", text)
}

func (test *ComposerWordsServiceTest) TestModeTextNamesSeveralOutsidePeople() {
	text := test.words().ModeText([]Recipient{test.ready("Carol Diaz", "external", "email"), test.ready("Finn Moss", "otherDomain", "email")}, "example.com")
	Assert.Equals("This is an email. Carol Diaz and Finn Moss are outside example.com.", text)
}

func (test *ComposerWordsServiceTest) TestModeTextWhenEveryoneIsInside() {
	text := test.words().ModeText([]Recipient{test.ready("Bo Lee", "member", "email")}, "example.com")
	Assert.Equals("This is an email. Everyone gets it in their mailbox.", text)
}

func (test *ComposerWordsServiceTest) TestModeTextForAMixedSend() {
	text := test.words().ModeText([]Recipient{test.ready("Bo Lee", "member", "chat"), test.ready("Carol Diaz", "external", "email")}, "example.com")
	Assert.Equals("This is a chat and an email. Chat stays inside example.com, so Carol Diaz gets an email.", text)
}

func (test *ComposerWordsServiceTest) TestFirstNameTakesTheFirstWord() {
	Assert.Equals("Bo", test.words().FirstName("Bo Lee"))
}

func (test *ComposerWordsServiceTest) TestFirstNameOfNothingIsEmpty() {
	Assert.Equals("", test.words().FirstName(""))
}

func (test *ComposerWordsServiceTest) TestInitialsTakeTwoLetters() {
	Assert.Equals("BL", test.words().Initials("Bo Van Lee"))
}

func (test *ComposerWordsServiceTest) TestInitialsOfOneWord() {
	Assert.Equals("C", test.words().Initials("carol@partner.org"))
}

func (test *ComposerWordsServiceTest) TestInitialsOfNothing() {
	Assert.Equals("", test.words().Initials(""))
}
