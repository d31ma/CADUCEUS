// @Test(SearchTestSetup, {"partition":"tests"})
type SearchServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SearchServiceTest) search() *SearchService {
	search := &SearchService{queries: SearchTestSetup{}.Queries(test.behaviors), recent: &SearchRecentService{}, shell: &ShellService{accounts: &SessionAccountService{}}}
	search.Start()
	return search
}

func (test *SearchServiceTest) paletteFound(query string) *SearchService {
	search := test.search()
	search.OpenPalette()
	search.SearchPaletteFor(query)
	return search
}

func (test *SearchServiceTest) pageFound(query string) *SearchService {
	search := test.search()
	search.Query = query
	search.Search()
	return search
}

func (test *SearchServiceTest) TestStartIsIdle() {
	search := test.search()
	Assert.Equals("idle", search.PaletteState)
	Assert.Equals("idle", search.PageState)
	Assert.IsTrue(!search.PaletteOpen)
}

func (test *SearchServiceTest) TestOpenPaletteShowsIt() {
	search := test.search()
	search.OpenPalette()
	Assert.IsTrue(search.PaletteOpen)
}

func (test *SearchServiceTest) TestClosePaletteHidesIt() {
	search := test.search()
	search.OpenPalette()
	search.ClosePalette()
	Assert.IsTrue(!search.PaletteOpen)
}

func (test *SearchServiceTest) TestSearchPaletteFindsAndRemembersTheSearch() {
	search := test.search()
	search.PaletteQuery = " invoice "
	search.SearchPalette()
	Assert.Equals("results", search.PaletteState)
	Assert.Equals([]string{"invoice"}, search.RecentSearches)
	Assert.Equals("invoice", search.paletteFilter().Query)
	Assert.Equals("5", search.paletteFilter().Limit)
}

func (test *SearchServiceTest) TestSearchPaletteWithoutTextStaysIdle() {
	search := test.search()
	search.PaletteQuery = "  "
	search.SearchPalette()
	Assert.Equals("idle", search.PaletteState)
	Assert.Equals(0, len(search.RecentSearches))
}

func (test *SearchServiceTest) TestSearchPaletteShowsTheQuickResults() {
	search := test.paletteFound("invoice")
	Assert.Equals("results", search.PaletteState)
	Assert.Equals(2, len(search.PaletteRows))
	Assert.Equals(SearchHitPart{Text: "invoice", Match: true}, search.PaletteRows[0].Parts[5])
}

func (test *SearchServiceTest) TestSearchPaletteShowsTheEmptyState() {
	Assert.Equals("empty", test.paletteFound("nothing").PaletteState)
}

func (test *SearchServiceTest) TestSearchPaletteShowsTheErrorState() {
	search := test.paletteFound("broken")
	Assert.Equals("error", search.PaletteState)
	Assert.Equals(0, len(search.PaletteRows))
}

func (test *SearchServiceTest) TestClearPaletteQueryReturnsToRecentSearches() {
	search := test.paletteFound("invoice")
	search.ClearPaletteQuery()
	Assert.Equals("", search.PaletteQuery)
	Assert.Equals("idle", search.PaletteState)
	Assert.Equals(0, len(search.PaletteRows))
	Assert.Equals([]string{"invoice"}, search.RecentSearches)
}

func (test *SearchServiceTest) TestSearchPaletteForRunsARecentSearchAgain() {
	search := test.search()
	search.RecentSearches = []string{"march", "invoice"}
	search.SearchPaletteFor("invoice")
	Assert.Equals("invoice", search.PaletteQuery)
	Assert.Equals("results", search.PaletteState)
	Assert.Equals([]string{"invoice", "march"}, search.RecentSearches)
}

func (test *SearchServiceTest) TestClearRecentForgetsEverySearch() {
	search := test.search()
	search.RecentSearches = []string{"march", "invoice"}
	search.ClearRecent()
	Assert.Equals(0, len(search.RecentSearches))
}

func (test *SearchServiceTest) TestOpenClosesThePalette() {
	search := test.paletteFound("invoice")
	search.Open(search.PaletteRows[0])
	Assert.IsTrue(!search.PaletteOpen)
}

func (test *SearchServiceTest) TestSeeAllMovesThePaletteSearchToThePage() {
	search := test.search()
	search.OpenPalette()
	search.PaletteQuery = "invoice"
	search.SeeAll()
	Assert.Equals("invoice", search.Query)
	Assert.Equals("results", search.PageState)
	Assert.IsTrue(!search.PaletteOpen)
	Assert.Equals("search", search.shell.Page)
	Assert.Equals("main", search.shell.Pane)
}

func (test *SearchServiceTest) TestSearchWithoutTextStaysIdle() {
	search := test.search()
	search.Search()
	Assert.Equals("idle", search.PageState)
	Assert.Equals("", search.SearchedQuery)
}

func (test *SearchServiceTest) TestSearchShowsResultsNewestFirstWithMore() {
	search := test.pageFound("invoice")
	Assert.Equals("results", search.PageState)
	Assert.Equals("m-1", search.PageRows[0].MessageId)
	Assert.Equals("page-2", search.NextCursor)
	Assert.Equals("invoice", search.SearchedQuery)
	Assert.Equals("20", search.searched.Limit)
}

func (test *SearchServiceTest) TestSearchShowsTheEmptyState() {
	search := test.pageFound("nothing")
	Assert.Equals("empty", search.PageState)
	Assert.Equals("nothing", search.SearchedQuery)
}

func (test *SearchServiceTest) TestLoadMoreAddsTheNextPage() {
	search := test.pageFound("invoice")
	search.LoadMore()
	Assert.Equals(3, len(search.PageRows))
	Assert.Equals("m-3", search.PageRows[2].MessageId)
	Assert.Equals("", search.NextCursor)
	Assert.Equals("results", search.MoreState)
}

func (test *SearchServiceTest) TestLoadMoreFailureKeepsTheRowsAndTheCursor() {
	search := test.pageFound("invoice")
	search.searched.Query = "broken"
	search.LoadMore()
	Assert.Equals(2, len(search.PageRows))
	Assert.Equals("page-2", search.NextCursor)
	Assert.Equals("error", search.MoreState)
}

func (test *SearchServiceTest) TestScopeToSearchesOneConversation() {
	search := test.search()
	search.Query = "invoice"
	search.ScopeTo(SearchHitScopeRequested{ConversationId: "c-1", Title: "support"})
	Assert.Equals("c-1", search.pageFilter().ConversationId)
	Assert.Equals("support", search.ConversationTitle)
	Assert.Equals("results", search.PageState)
}

func (test *SearchServiceTest) TestClearScopeSearchesEveryConversation() {
	search := test.search()
	search.ScopeTo(SearchHitScopeRequested{ConversationId: "c-1", Title: "support"})
	search.ClearScope()
	Assert.Equals("", search.ConversationId)
	Assert.Equals("", search.ConversationTitle)
}

func (test *SearchServiceTest) TestToggleAttachmentTurnsTheFilterOnAndOff() {
	search := test.search()
	search.ToggleAttachment()
	Assert.IsTrue(search.pageFilter().HasAttachment)
	search.ToggleAttachment()
	Assert.IsTrue(!search.HasAttachment)
}

func (test *SearchServiceTest) TestToggleEmailTurnsTheFilterOnAndOff() {
	search := test.search()
	search.ToggleEmail()
	Assert.IsTrue(search.pageFilter().IsEmail)
	search.ToggleEmail()
	Assert.IsTrue(!search.IsEmail)
}

func (test *SearchServiceTest) TestSearchWithReadsTheSubmittedFields() {
	search := test.search()
	search.SearchWith(" invoice ", " bo ", "2026-11-01", "2026-11-10")
	Assert.Equals(SearchHitFilter{Query: "invoice", Author: "bo", From: "2026-11-01", To: "2026-11-10", Limit: "20"}, search.searched)
	Assert.Equals("results", search.PageState)
	Assert.Equals([]string{"invoice"}, search.RecentSearches)
}

func (test *SearchServiceTest) TestDomainChangedClearsTheOldDomainResults() {
	search := test.pageFound("invoice")
	search.DomainChanged(SessionDomainChanged{DomainId: "example.com"})
	search.ScopeTo(SearchHitScopeRequested{ConversationId: "c-1", Title: "support"})
	search.DomainChanged(SessionDomainChanged{DomainId: "example.org"})
	Assert.Equals("idle", search.PageState)
	Assert.Equals(0, len(search.PageRows))
	Assert.Equals("", search.ConversationId)
	Assert.Equals("invoice", search.Query)
}

func (test *SearchServiceTest) TestDomainChangedKeepsResultsOfTheSameDomain() {
	search := test.pageFound("invoice")
	search.DomainChanged(SessionDomainChanged{DomainId: "example.com"})
	search.DomainChanged(SessionDomainChanged{DomainId: "example.com"})
	Assert.Equals("results", search.PageState)
	Assert.Equals(2, len(search.PageRows))
}

func (test *SearchServiceTest) TestEveryFilterReachesTheSearch() {
	search := test.search()
	search.SearchWith("filtered", " bo ", "2026-11-01", "2026-11-10")
	search.ToggleAttachment()
	search.ToggleEmail()
	search.ScopeTo(SearchHitScopeRequested{ConversationId: "c-1", Title: "support"})
	Assert.Equals("results", search.PageState)
	Assert.Equals("m-9", search.PageRows[0].MessageId)
}

func (test *SearchServiceTest) TestClearFiltersKeepsTheQuery() {
	search := test.search()
	search.Query = "invoice"
	search.Author = "bo"
	search.From = "2026-11-01"
	search.To = "2026-11-10"
	search.ToggleAttachment()
	search.ToggleEmail()
	search.ScopeTo(SearchHitScopeRequested{ConversationId: "c-1", Title: "support"})
	search.ClearFilters()
	Assert.Equals(SearchHitFilter{Query: "invoice", Limit: "20"}, search.pageFilter())
	Assert.Equals("", search.ConversationTitle)
	Assert.Equals("results", search.PageState)
}

func (test *SearchServiceTest) TestClearQueryEmptiesThePage() {
	search := test.pageFound("invoice")
	search.ClearQuery()
	Assert.Equals("", search.Query)
	Assert.Equals("idle", search.PageState)
	Assert.Equals(0, len(search.PageRows))
	Assert.Equals("", search.NextCursor)
}
