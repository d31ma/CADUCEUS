// @Test(ChannelTestSetup, {"partition":"tests"})
type ChannelAddressServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	channelsClient *ChannelsClient
	// @Fake
	identityClient *IdentityClient
}

func (test *ChannelAddressServiceTest) editing(channelId string) *ChannelAddressService {
	session := &AdminSessionService{}
	test.construct(session)
	list := &ChannelListService{session: session, problems: &AdminProblemService{}}
	test.construct(list)
	list.Opened(AdminViewOpened{View: "channels", DomainId: "example.com"})
	dialog := &ChannelAddressService{channelList: list, session: session, problems: &AdminProblemService{}}
	test.construct(dialog)
	dialog.Edit(channelId)
	return dialog
}

func (test *ChannelAddressServiceTest) saved(channelId string, localPart string) *ChannelAddressService {
	dialog := test.editing(channelId)
	dialog.LocalPart = localPart
	dialog.Save()
	return dialog
}

func (test *ChannelAddressServiceTest) TestEditOpensAnEmptyAddressForAChannelWithoutOne() {
	dialog := test.editing("general-id")
	Assert.Equals(true, dialog.Open)
	Assert.Equals(false, dialog.Editing)
	Assert.Equals("", dialog.LocalPart)
	Assert.Equals("example.com", dialog.Domain)
	Assert.Equals("Email address for #general", dialog.Title)
}

func (test *ChannelAddressServiceTest) TestEditShowsTheCurrentAddress() {
	dialog := test.editing("support-id")
	Assert.Equals(true, dialog.Editing)
	Assert.Equals("support", dialog.LocalPart)
	Assert.Equals("support", dialog.CurrentLocalPart)
	Assert.Equals("This is the current address of #support.", dialog.CurrentText)
}

func (test *ChannelAddressServiceTest) TestEditExplainsTheAddressWithTheDomain() {
	dialog := test.editing("support-id")
	Assert.Equals("Ends with @example.com. The address must not belong to a person, an alias or another channel.", dialog.Help)
	Assert.Equals("Mail sent to this address shows in #support as an email conversation. Members reply as email. Chat in #support stays inside example.com.", dialog.MailText)
	Assert.Equals("", dialog.Problem)
}

func (test *ChannelAddressServiceTest) TestCancelClosesTheDialog() {
	dialog := test.editing("general-id")
	dialog.Cancel()
	Assert.Equals(false, dialog.Open)
}

func (test *ChannelAddressServiceTest) TestSaveGivesTheChannelAnAddressInItsDomain() {
	dialog := test.saved("general-id", "news")
	Assert.Equals("", dialog.Problem)
	Assert.Equals(false, dialog.Open)
	Assert.Equals(false, dialog.Busy)
	Assert.Equals("news@example.com", dialog.channelList.Rows[0].EmailAddress)
	Assert.Equals("Edit address", dialog.channelList.Rows[0].AddressAction)
}

func (test *ChannelAddressServiceTest) TestSaveAcceptsDotsHyphensAndUnderscores() {
	dialog := test.saved("general-id", "team.news-2_x")
	Assert.Equals(false, dialog.Open)
	Assert.Equals("team.news-2_x@example.com", dialog.channelList.Rows[0].EmailAddress)
}

func (test *ChannelAddressServiceTest) TestSaveAcceptsSixtyFourCharacters() {
	dialog := test.saved("general-id", "abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij1234")
	Assert.Equals("", dialog.Problem)
	Assert.Equals(false, dialog.Open)
}

func (test *ChannelAddressServiceTest) TestSaveRefusesAnEmptyAddress() {
	dialog := test.saved("general-id", "")
	Assert.Equals("Enter the part of the address before @example.com.", dialog.Problem)
	Assert.Equals(true, dialog.Open)
}

func (test *ChannelAddressServiceTest) TestSaveRefusesMoreThanSixtyFourCharacters() {
	dialog := test.saved("general-id", "abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij12345")
	Assert.Equals("Use at most 64 characters before the @.", dialog.Problem)
	Assert.Equals(true, dialog.Open)
}

func (test *ChannelAddressServiceTest) TestSaveRefusesUppercaseLetters() {
	dialog := test.saved("general-id", "News")
	Assert.Equals("Use only lowercase letters, digits, dots, hyphens and underscores.", dialog.Problem)
}

func (test *ChannelAddressServiceTest) TestSaveRefusesAnotherDomain() {
	dialog := test.saved("general-id", "news@example.org")
	Assert.Equals("Use only lowercase letters, digits, dots, hyphens and underscores.", dialog.Problem)
	Assert.Equals("", dialog.channelList.Rows[0].EmailAddress)
}

func (test *ChannelAddressServiceTest) TestSaveRefusesALeadingDot() {
	dialog := test.saved("general-id", ".news")
	Assert.Equals("The address cannot start or end with a dot.", dialog.Problem)
}

func (test *ChannelAddressServiceTest) TestSaveRefusesATrailingDot() {
	dialog := test.saved("general-id", "news.")
	Assert.Equals("The address cannot start or end with a dot.", dialog.Problem)
}

func (test *ChannelAddressServiceTest) TestSaveSaysWhenTheAddressIsTaken() {
	dialog := test.saved("general-id", "bo")
	Assert.Equals("This address already belongs to a member, alias or another channel.", dialog.Problem)
	Assert.Equals(true, dialog.Open)
	Assert.Equals("", dialog.channelList.Rows[0].EmailAddress)
}

func (test *ChannelAddressServiceTest) TestSaveSaysOnlyADomainAdminMayChangeIt() {
	dialog := test.saved("locked-id", "board2")
	Assert.Equals("Only a domain admin can change channel addresses.", dialog.Problem)
	Assert.Equals(true, dialog.Open)
}

func (test *ChannelAddressServiceTest) TestSaveSaysWhenTheChannelIsGone() {
	dialog := test.saved("gone-id", "old")
	Assert.Equals("This channel no longer exists.", dialog.Problem)
	Assert.Equals(true, dialog.Open)
}

func (test *ChannelAddressServiceTest) TestSaveSaysWhenTheServerIsUnreachable() {
	dialog := test.saved("general-id", "down")
	Assert.Equals("Mesail could not reach the server. Check your connection, then try again.", dialog.Problem)
	Assert.Equals(false, dialog.Busy)
	Assert.Equals(true, dialog.Open)
}

func (test *ChannelAddressServiceTest) TestRemoveTakesTheAddressAway() {
	dialog := test.editing("support-id")
	dialog.Remove()
	Assert.Equals(false, dialog.Open)
	Assert.Equals(false, dialog.channelList.Rows[1].HasAddress)
	Assert.Equals("Add address", dialog.channelList.Rows[1].AddressAction)
}

func (test *ChannelAddressServiceTest) TestRemoveSaysOnlyADomainAdminMayChangeIt() {
	dialog := test.editing("locked-id")
	dialog.Remove()
	Assert.Equals("Only a domain admin can change channel addresses.", dialog.Problem)
	Assert.Equals(true, dialog.Open)
	Assert.Equals("board@example.com", dialog.channelList.Rows[5].EmailAddress)
}
