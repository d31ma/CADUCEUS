// @Test(ChannelTestSetup, {"partition":"tests"})
type ChannelListServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	channelsClient *ChannelsClient
	// @Fake
	identityClient *IdentityClient
}

func (test *ChannelListServiceTest) opened() *ChannelListService {
	list := &ChannelListService{Search: "old", Show: "archived", session: test.session(), problems: &AdminProblemService{}}
	test.construct(list)
	list.Opened(AdminViewOpened{View: "channels", DomainId: "example.com"})
	return list
}

func (test *ChannelListServiceTest) session() *AdminSessionService {
	session := &AdminSessionService{}
	test.construct(session)
	return session
}

func (test *ChannelListServiceTest) TestOpenedListsEveryChannelOfTheDomain() {
	list := test.opened()
	Assert.Equals("ready", list.State)
	Assert.Equals("example.com", list.Domain)
	Assert.Equals(6, len(list.Rows))
	Assert.Equals("general", list.Rows[0].Name)
	Assert.Equals("Company news", list.Rows[0].Topic)
	Assert.Equals("general-id", list.Rows[0].ChannelId)
}

func (test *ChannelListServiceTest) TestOpenedForgetsTheOldSearchAndFilter() {
	list := test.opened()
	Assert.Equals("", list.Search)
	Assert.Equals("all", list.Show)
}

func (test *ChannelListServiceTest) TestOpenedCountsAddressesAndArchivedChannels() {
	list := test.opened()
	Assert.Equals("6 channels", list.Heading)
	Assert.Equals("2 with an email address · 1 archived", list.Summary)
	Assert.Equals("Showing 6 of 6.", list.Showing)
}

func (test *ChannelListServiceTest) TestAPublicChannelShowsAnOutlineTag() {
	list := test.opened()
	Assert.Equals("Public", list.Rows[0].VisibilityText)
	Assert.Equals("ms-tag ms-tag--outline", list.Rows[0].VisibilityTag)
	Assert.Equals("ms-icon ms-i-globe", list.Rows[0].VisibilityIcon)
	Assert.Equals("ms-icon ms-i-hash ms-icon--sm", list.Rows[0].NameIcon)
}

func (test *ChannelListServiceTest) TestAPrivateChannelShowsALock() {
	list := test.opened()
	Assert.Equals("Private", list.Rows[2].VisibilityText)
	Assert.Equals("ms-tag", list.Rows[2].VisibilityTag)
	Assert.Equals("ms-icon ms-i-lock", list.Rows[2].VisibilityIcon)
	Assert.Equals("ms-icon ms-i-lock ms-icon--sm", list.Rows[2].NameIcon)
}

func (test *ChannelListServiceTest) TestAChannelWithoutAnAddressOffersToAddOne() {
	list := test.opened()
	Assert.Equals(false, list.Rows[0].HasAddress)
	Assert.Equals("Add address", list.Rows[0].AddressAction)
	Assert.Equals("Add address for #general", list.Rows[0].AddressLabel)
	Assert.Equals("Archive #general", list.Rows[0].ArchiveLabel)
}

func (test *ChannelListServiceTest) TestAChannelWithAnAddressOffersToEditIt() {
	list := test.opened()
	Assert.Equals(true, list.Rows[1].HasAddress)
	Assert.Equals("support@example.com", list.Rows[1].EmailAddress)
	Assert.Equals("Edit address", list.Rows[1].AddressAction)
	Assert.Equals("Edit address for #support", list.Rows[1].AddressLabel)
}

func (test *ChannelListServiceTest) TestAnArchivedChannelIsMuted() {
	list := test.opened()
	Assert.Equals(true, list.Rows[3].Archived)
	Assert.Equals("is-muted", list.Rows[3].RowClass)
	Assert.Equals("Unarchive #launch-q3", list.Rows[3].RestoreLabel)
	Assert.Equals("", list.Rows[0].RowClass)
}

func (test *ChannelListServiceTest) TestSearchForFindsAChannelByName() {
	list := test.opened()
	list.SearchFor("SUPP")
	Assert.Equals(1, len(list.Rows))
	Assert.Equals("support", list.Rows[0].Name)
	Assert.Equals("Showing 1 of 6.", list.Showing)
}

func (test *ChannelListServiceTest) TestSearchForAcceptsAChannelSign() {
	list := test.opened()
	list.SearchFor(" #design ")
	Assert.Equals(1, len(list.Rows))
	Assert.Equals("design", list.Rows[0].Name)
}

func (test *ChannelListServiceTest) TestSearchForFindsAChannelByAddress() {
	list := test.opened()
	list.SearchFor("board@")
	Assert.Equals(1, len(list.Rows))
	Assert.Equals("board", list.Rows[0].Name)
}

func (test *ChannelListServiceTest) TestSearchWithoutAMatchShowsNoResults() {
	list := test.opened()
	list.SearchFor("zoe")
	Assert.Equals("noResults", list.State)
	Assert.Equals("No channels match “zoe”", list.NoResultsTitle)
	Assert.Equals("6 channels", list.Heading)
}

func (test *ChannelListServiceTest) TestShowOnlyPublicChannels() {
	list := test.opened()
	list.ShowOnly("public")
	Assert.Equals(4, len(list.Rows))
	Assert.Equals("general", list.Rows[0].Name)
}

func (test *ChannelListServiceTest) TestShowOnlyPrivateChannels() {
	list := test.opened()
	list.ShowOnly("private")
	Assert.Equals(2, len(list.Rows))
	Assert.Equals("design", list.Rows[0].Name)
	Assert.Equals("board", list.Rows[1].Name)
}

func (test *ChannelListServiceTest) TestShowOnlyChannelsWithAnEmailAddress() {
	list := test.opened()
	list.ShowOnly("email")
	Assert.Equals(2, len(list.Rows))
	Assert.Equals("support", list.Rows[0].Name)
}

func (test *ChannelListServiceTest) TestShowOnlyArchivedChannels() {
	list := test.opened()
	list.ShowOnly("archived")
	Assert.Equals(1, len(list.Rows))
	Assert.Equals("launch-q3", list.Rows[0].Name)
}

func (test *ChannelListServiceTest) TestAFilterWithoutAMatchNamesTheFilter() {
	general := Channel{ChannelId: "general-id", Name: "general", Visibility: "public"}
	list := &ChannelListService{channels: []Channel{general}}
	list.ShowOnly("archived")
	Assert.Equals("noResults", list.State)
	Assert.Equals("No channels match this filter", list.NoResultsTitle)
}

func (test *ChannelListServiceTest) TestClearFiltersShowsEveryChannelAgain() {
	list := test.opened()
	list.ShowOnly("private")
	list.SearchFor("zoe")
	list.ClearFilters()
	Assert.Equals("ready", list.State)
	Assert.Equals(6, len(list.Rows))
	Assert.Equals("", list.Search)
	Assert.Equals("all", list.Show)
}

func (test *ChannelListServiceTest) TestRefreshLoadsTheChannelsAgainAndKeepsTheFilter() {
	list := test.opened()
	list.ShowOnly("email")
	list.Refresh()
	Assert.Equals("ready", list.State)
	Assert.Equals(2, len(list.Rows))
}

func (test *ChannelListServiceTest) TestFindGivesTheLoadedChannel() {
	list := test.opened()
	Assert.Equals("support@example.com", list.Find("support-id").EmailAddress)
	Assert.Equals("", list.Find("missing-id").ChannelId)
}

func (test *ChannelListServiceTest) TestReplaceUpdatesTheRowInPlace() {
	list := test.opened()
	list.Replace(Channel{ChannelId: "general-id", Name: "general", Visibility: "public", EmailAddress: "news@example.com"})
	Assert.Equals("general", list.Rows[0].Name)
	Assert.Equals("news@example.com", list.Rows[0].EmailAddress)
	Assert.Equals("3 with an email address · 1 archived", list.Summary)
	Assert.Equals(6, len(list.Rows))
}
