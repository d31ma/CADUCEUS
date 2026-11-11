// @Test(ConversationTestSetup, {"partition":"tests"})
type ConversationServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ConversationServiceTest) build() *ConversationService {
	service := &ConversationService{
		requests:    test.requests(),
		messageList: &ConversationMessageService{reactions: &ConversationReactionService{}},
		views: &ConversationViewService{
			times:       &ConversationTimeService{},
			reactions:   &ConversationReactionService{},
			attachments: &ConversationAttachmentService{},
			deliveries:  &ConversationDeliveryService{},
		},
		shellAnnounce: &ShellAnnounceService{},
	}
	service.Start()
	return service
}

func (test *ConversationServiceTest) requests() *ConversationRequestService {
	BrowserFakes{}.Answer("clock", "now", 1794340800000)
	BrowserFakes{}.Answer("clock", "timeZone", "UTC")
	accounts := &SessionAccountService{}
	accounts.Add(Me{MemberId: "ana", DomainId: "example.com", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.com"})
	tokens := &SessionTokenService{tokens: map[string]SessionTokens{"example.com": {AccessToken: "access-1", ExpiresAtMs: 1900000000000}}}
	return &ConversationRequestService{
		messaging: NewMessagingClientWith(test.behaviors),
		token:     &ConversationTokenService{session: &SessionService{tokens: tokens, accounts: accounts}, accounts: accounts},
	}
}

func (test *ConversationServiceTest) opened(conversationId string, section string) *ConversationService {
	service := test.build()
	service.Opened(ConversationOpened{DomainId: "example.com", ConversationId: conversationId, Title: "general", Section: section, UnreadCount: 1})
	return service
}

func (test *ConversationServiceTest) general() *ConversationService {
	return test.opened("cnv_general", "channels")
}

func (test *ConversationServiceTest) TestStartIsIdle() {
	service := test.build()
	Assert.Equals("idle", service.State)
	Assert.Equals(0, len(service.Messages))
}

func (test *ConversationServiceTest) TestOpenedShowsTimesInTheBrowserTimeZone() {
	service := test.build()
	BrowserFakes{}.Answer("clock", "timeZone", "America/New_York")
	service.Opened(ConversationOpened{DomainId: "example.com", ConversationId: "cnv_general", Title: "general", Section: "channels", UnreadCount: 1})
	Assert.Equals("04:00", service.Messages[0].Time)
	Assert.Equals("09:05", service.Messages[1].Time)
	Assert.Equals("Today", service.Messages[1].DayLabel)
}

func (test *ConversationServiceTest) TestOpenedShowsUniversalTimeWhenTheTimeZoneIsUnknown() {
	service := test.build()
	BrowserFakes{}.Answer("clock", "timeZone", "Mars/Olympus_Mons")
	service.Opened(ConversationOpened{DomainId: "example.com", ConversationId: "cnv_general", Title: "general", Section: "channels", UnreadCount: 1})
	Assert.Equals("14:05", service.Messages[1].Time)
}

func (test *ConversationServiceTest) TestOpenedLoadsTheFirstPageOldestFirst() {
	service := test.general()
	Assert.Equals("ready", service.State)
	Assert.Equals("example.com", service.DomainId)
	Assert.Equals("cnv_general", service.ConversationId)
	Assert.Equals(3, len(service.Messages))
	Assert.Equals("m-1", service.Messages[0].MessageId)
	Assert.IsTrue(service.Messages[2].NewDivider)
	Assert.IsTrue(service.HasOlder)
	Assert.Equals("", service.Problem)
}

func (test *ConversationServiceTest) TestOpenedForAnotherDomainIsIgnored() {
	service := test.build()
	service.Opened(ConversationOpened{DomainId: "example.org", ConversationId: "cnv_general"})
	Assert.Equals("idle", service.State)
	Assert.Equals("", service.ConversationId)
}

func (test *ConversationServiceTest) TestOpenedEmptyConversationIsEmpty() {
	Assert.Equals("empty", test.opened("cnv_empty", "direct").State)
}

func (test *ConversationServiceTest) TestOpenedFailureShowsAPlainProblem() {
	service := test.opened("cnv_missing", "direct")
	Assert.Equals("error", service.State)
	Assert.Equals("We could not load this conversation. Check your connection and try again.", service.Problem)
}

func (test *ConversationServiceTest) TestRetryLoadsAgain() {
	service := test.general()
	service.State = "error"
	service.Retry()
	Assert.Equals("ready", service.State)
	Assert.Equals(3, len(service.Messages))
}

func (test *ConversationServiceTest) TestDomainChangedClearsEverything() {
	service := test.general()
	service.DomainChanged(SessionDomainChanged{DomainId: "example.org"})
	Assert.Equals("idle", service.State)
	Assert.Equals("", service.ConversationId)
	Assert.Equals(0, len(service.Messages))
	Assert.Equals(0, len(service.raw))
}

func (test *ConversationServiceTest) TestDomainChangedForTheSameDomainKeepsTheConversation() {
	service := test.general()
	service.DomainChanged(SessionDomainChanged{DomainId: "example.com"})
	Assert.Equals("ready", service.State)
	Assert.Equals("cnv_general", service.ConversationId)
}

func (test *ConversationServiceTest) TestLoadOlderAddsOlderMessages() {
	service := test.general()
	service.LoadOlder()
	Assert.Equals(4, len(service.Messages))
	Assert.Equals("m-0", service.Messages[0].MessageId)
	Assert.IsTrue(!service.HasOlder)
	Assert.IsTrue(!service.LoadingOlder)
}

func (test *ConversationServiceTest) TestLoadOlderFailureKeepsTheMessages() {
	service := test.opened("cnv_missing", "direct")
	service.ConversationId = "cnv_other"
	service.LoadOlder()
	Assert.Equals("We could not load this conversation. Check your connection and try again.", service.Problem)
	Assert.Equals(0, len(service.Messages))
}

func (test *ConversationServiceTest) TestStartEditCopiesTheBody() {
	service := test.general()
	service.StartEdit("m-2")
	Assert.Equals("m-2", service.EditingId)
	Assert.Equals("Lunch at noon?", service.EditBody)
	Assert.IsTrue(service.Messages[1].Editing)
}

func (test *ConversationServiceTest) TestSaveEditReplacesTheMessage() {
	service := test.general()
	service.StartEdit("m-2")
	service.SaveEdit("Lunch at one?")
	Assert.Equals("", service.EditingId)
	Assert.Equals("", service.Problem)
	Assert.IsTrue(service.Messages[1].Edited)
	Assert.IsTrue(!service.Messages[1].Editing)
}

func (test *ConversationServiceTest) TestSaveEditWithAnEmptyBodyKeepsEditing() {
	service := test.general()
	service.StartEdit("m-2")
	service.SaveEdit("   ")
	Assert.Equals("m-2", service.EditingId)
	Assert.Equals("", service.Problem)
}

func (test *ConversationServiceTest) TestSaveEditFailureKeepsEditing() {
	service := test.general()
	service.StartEdit("m-3")
	service.SaveEdit("Thanks again")
	Assert.Equals("m-3", service.EditingId)
	Assert.Equals("We could not save your change. Try again.", service.Problem)
}

func (test *ConversationServiceTest) TestCancelEditStopsEditing() {
	service := test.general()
	service.StartEdit("m-2")
	service.CancelEdit()
	Assert.Equals("", service.EditingId)
	Assert.Equals("", service.EditBody)
}

func (test *ConversationServiceTest) TestAskDeleteWarnsAboutEmail() {
	service := test.general()
	service.AskDelete("m-2")
	Assert.Equals("m-2", service.DeletingId)
	Assert.Equals("carol@outside.test already got this as an email. Email cannot be recalled.", service.DeleteWarning)
}

func (test *ConversationServiceTest) TestCancelDeleteClearsTheQuestion() {
	service := test.general()
	service.AskDelete("m-2")
	service.CancelDelete()
	Assert.Equals("", service.DeletingId)
	Assert.Equals("", service.DeleteWarning)
}

func (test *ConversationServiceTest) TestConfirmDeleteRefetches() {
	service := test.general()
	service.AskDelete("m-2")
	service.ConfirmDelete()
	Assert.Equals("", service.DeletingId)
	Assert.Equals("", service.Problem)
	Assert.Equals(3, len(service.Messages))
}

func (test *ConversationServiceTest) TestConfirmDeleteFailureShowsAProblem() {
	service := test.general()
	service.AskDelete("m-1")
	service.ConfirmDelete()
	Assert.Equals("We could not delete the message. Try again.", service.Problem)
}

func (test *ConversationServiceTest) TestToggleReactionAddsMine() {
	service := test.general()
	service.OpenPicker("m-3")
	service.ToggleReaction("m-3", "thumbs_up")
	Assert.Equals("", service.Problem)
	Assert.Equals("", service.PickerForId)
}

func (test *ConversationServiceTest) TestToggleReactionRemovesMine() {
	service := test.general()
	service.raw[0].MyReactions = []string{"thumbs_up"}
	service.ToggleReaction("m-3", "thumbs_up")
	Assert.Equals("", service.Problem)
}

func (test *ConversationServiceTest) TestToggleReactionFailureShowsAProblem() {
	service := test.general()
	service.ToggleReaction("m-1", "heart")
	Assert.Equals("We could not update the reaction. Try again.", service.Problem)
}

func (test *ConversationServiceTest) TestOpenPickerListsTheChoices() {
	service := test.general()
	service.OpenPicker("m-3")
	Assert.Equals("m-3", service.PickerForId)
	Assert.Equals(12, len(service.ReactionChoices))
	Assert.Equals("Add thumbs up", service.ReactionChoices[0].Label)
	service.ClosePicker()
	Assert.Equals("", service.PickerForId)
	Assert.Equals(0, len(service.ReactionChoices))
}

func (test *ConversationServiceTest) TestAllowImagesShowsTheSendersImages() {
	service := test.opened("cnv_carol", "email")
	Assert.Equals(1, service.Messages[0].Email.BlockedImages)
	service.AllowImages("Carol@Outside.test")
	service.AllowImages("carol@outside.test")
	Assert.Equals([]string{"carol@outside.test"}, service.AllowedSenders["example.com"])
	Assert.IsTrue(service.Messages[0].Email.ImagesAllowed)
}

func (test *ConversationServiceTest) TestToggleQuotedShowsAndHides() {
	service := test.opened("cnv_carol", "email")
	service.ToggleQuoted("m-9")
	Assert.IsTrue(service.Messages[0].ShowQuoted)
	service.ToggleQuoted("m-9")
	Assert.IsTrue(!service.Messages[0].ShowQuoted)
}

func (test *ConversationServiceTest) TestFetchOriginalGivesTheLink() {
	service := test.opened("cnv_carol", "email")
	service.FetchOriginal("m-9")
	Assert.Equals("m-9", service.OriginalForId)
	Assert.Equals("https://files.example.com/m-9.eml", service.OriginalUrl)
}

func (test *ConversationServiceTest) TestFetchOriginalFailureShowsAProblem() {
	service := test.general()
	service.FetchOriginal("m-1")
	Assert.Equals("", service.OriginalUrl)
	Assert.Equals("We could not get the original email. Try again.", service.Problem)
}

func (test *ConversationServiceTest) TestOpenThreadKeepsTheConversation() {
	service := test.general()
	service.OpenThread("m-3")
	Assert.Equals("cnv_general", service.ConversationId)
}

func (test *ConversationServiceTest) TestLiveNewMessageIsAnnounced() {
	service := test.general()
	service.Live(RealtimeEvent{Type: "message.created", ConversationId: "cnv_general", MessageId: "m-3"})
	Assert.Equals("New message from Bo Lee", service.shellAnnounce.Message)
	Assert.Equals(3, len(service.Messages))
}

func (test *ConversationServiceTest) TestLiveMessageFromMeIsNotAnnounced() {
	service := test.general()
	service.Live(RealtimeEvent{Type: "message.created", ConversationId: "cnv_general", MessageId: "m-2"})
	Assert.Equals("", service.shellAnnounce.Message)
}

func (test *ConversationServiceTest) TestLiveForAnotherConversationIsIgnored() {
	service := test.general()
	service.Live(RealtimeEvent{Type: "message.created", ConversationId: "cnv_bo", MessageId: "m-3"})
	Assert.Equals("", service.shellAnnounce.Message)
}

func (test *ConversationServiceTest) TestLiveKeepsOlderPages() {
	service := test.general()
	service.LoadOlder()
	service.Live(RealtimeEvent{Type: "message.edited", ConversationId: "cnv_general", MessageId: "m-2"})
	Assert.Equals(4, len(service.Messages))
	Assert.Equals("", service.shellAnnounce.Message)
}

func (test *ConversationServiceTest) TestComposeRequestedClearsTheConversation() {
	service := test.general()
	service.ComposeRequested(ConversationComposeRequested{DomainId: "example.com"})
	Assert.Equals("idle", service.State)
	Assert.Equals(0, len(service.Messages))
}

func (test *ConversationServiceTest) TestStartHasNoActionsOpen() {
	service := test.build()
	Assert.Equals("", service.ActionsForId)
}

func (test *ConversationServiceTest) TestToggleActionsOpensForTheMessage() {
	service := test.general()
	service.ToggleActions("m-2")
	Assert.Equals("m-2", service.ActionsForId)
}

func (test *ConversationServiceTest) TestToggleActionsAgainClosesThem() {
	service := test.general()
	service.ToggleActions("m-2")
	service.ToggleActions("m-2")
	Assert.Equals("", service.ActionsForId)
}

func (test *ConversationServiceTest) TestToggleActionsMovesToAnotherMessage() {
	service := test.general()
	service.ToggleActions("m-2")
	service.ToggleActions("m-3")
	Assert.Equals("m-3", service.ActionsForId)
}

func (test *ConversationServiceTest) TestOpenedClosesTheActions() {
	service := test.general()
	service.ToggleActions("m-2")
	service.Opened(ConversationOpened{DomainId: "example.com", ConversationId: "cnv_general", Title: "general", Section: "channels", UnreadCount: 1})
	Assert.Equals("", service.ActionsForId)
}

func (test *ConversationServiceTest) TestComposeRequestedClosesTheActions() {
	service := test.general()
	service.ToggleActions("m-2")
	service.ComposeRequested(ConversationComposeRequested{DomainId: "example.com"})
	Assert.Equals("", service.ActionsForId)
}

func (test *ConversationServiceTest) TestStartEditClosesTheActions() {
	service := test.general()
	service.ToggleActions("m-2")
	service.StartEdit("m-2")
	Assert.Equals("", service.ActionsForId)
	Assert.Equals("m-2", service.EditingId)
}

func (test *ConversationServiceTest) TestAskDeleteClosesTheActions() {
	service := test.general()
	service.ToggleActions("m-2")
	service.AskDelete("m-2")
	Assert.Equals("", service.ActionsForId)
	Assert.Equals("m-2", service.DeletingId)
}

func (test *ConversationServiceTest) TestOpenPickerClosesTheActions() {
	service := test.general()
	service.ToggleActions("m-3")
	service.OpenPicker("m-3")
	Assert.Equals("", service.ActionsForId)
	Assert.Equals("m-3", service.PickerForId)
}

func (test *ConversationServiceTest) TestOpenThreadClosesTheActions() {
	service := test.general()
	service.ToggleActions("m-3")
	service.OpenThread("m-3")
	Assert.Equals("", service.ActionsForId)
}
