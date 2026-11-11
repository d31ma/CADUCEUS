// @Test(ConversationTestSetup, {"partition":"tests"})
type ConversationRequestServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ConversationRequestServiceTest) requests() *ConversationRequestService {
	BrowserFakes{}.Answer("clock", "now", 1794340800000)
	accounts := &SessionAccountService{}
	accounts.Add(Me{MemberId: "ana", DomainId: "example.com", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.com"})
	tokens := &SessionTokenService{tokens: map[string]SessionTokens{"example.com": {AccessToken: "access-1", ExpiresAtMs: 1900000000000}}}
	return &ConversationRequestService{
		messaging: NewMessagingClientWith(test.behaviors),
		token:     &ConversationTokenService{session: &SessionService{tokens: tokens, accounts: accounts}, accounts: accounts},
	}
}

func (test *ConversationRequestServiceTest) TestFirstPageAsksWithoutACursor() {
	page, failure := test.requests().FirstPage("cnv_general")
	Assert.Equals(nil, failure)
	Assert.Equals(3, len(page.Items))
	Assert.Equals("m-3", page.Items[0].MessageId)
	Assert.Equals("older", page.NextCursor)
}

func (test *ConversationRequestServiceTest) TestOlderPageSendsTheCursor() {
	page, failure := test.requests().OlderPage("cnv_general", "older")
	Assert.Equals(nil, failure)
	Assert.Equals("m-0", page.Items[0].MessageId)
	Assert.Equals("", page.NextCursor)
}

func (test *ConversationRequestServiceTest) TestUnknownConversationFails() {
	_, failure := test.requests().FirstPage("cnv_missing")
	Assert.IsTrue(failure != nil)
}

func (test *ConversationRequestServiceTest) TestEditAnswersTheChangedMessage() {
	message, failure := test.requests().Edit("m-2", "Lunch at one?")
	Assert.Equals(nil, failure)
	Assert.Equals("Lunch at one?", message.Body)
	Assert.Equals("2026-11-10T14:06:00Z", message.EditedAt)
}

func (test *ConversationRequestServiceTest) TestDeleteSucceeds() {
	Assert.Equals(nil, test.requests().Delete("m-2"))
}

func (test *ConversationRequestServiceTest) TestSetReactionAddsWhenNotMine() {
	Assert.Equals(nil, test.requests().SetReaction("m-3", "thumbs_up", false))
}

func (test *ConversationRequestServiceTest) TestSetReactionRemovesWhenMine() {
	Assert.Equals(nil, test.requests().SetReaction("m-3", "thumbs_up", true))
}

func (test *ConversationRequestServiceTest) TestMarkReadSendsTheNewestMessage() {
	Assert.Equals(nil, test.requests().MarkRead("cnv_general", "m-3"))
}

func (test *ConversationRequestServiceTest) TestMarkReadWithoutMessagesSendsNothing() {
	Assert.Equals(nil, test.requests().MarkRead("cnv_missing", ""))
}

func (test *ConversationRequestServiceTest) TestMarkReadFailureIsReturned() {
	Assert.IsTrue(test.requests().MarkRead("cnv_missing", "m-1") != nil)
}

func (test *ConversationRequestServiceTest) TestOriginalGivesTheDownloadLink() {
	download, failure := test.requests().Original("m-9")
	Assert.Equals(nil, failure)
	Assert.Equals("https://files.example.com/m-9.eml", download.Url)
}

func (test *ConversationRequestServiceTest) TestDomainAndAddressComeFromTheAccount() {
	requests := test.requests()
	Assert.Equals("example.com", requests.DomainId())
	Assert.Equals("ana@example.com", requests.MyAddress())
}
