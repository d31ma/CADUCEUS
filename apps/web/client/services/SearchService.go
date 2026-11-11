import "strings"

// @Service
type SearchService struct {
	PaletteOpen   bool
	PaletteQuery  string
	PaletteState  string
	PaletteRows   []SearchHitRow
	PageState     string
	PageRows      []SearchHitRow
	NextCursor    string
	MoreState     string
	SearchedQuery string
	searched      SearchHitFilter
	domainId      string
	// @LocalState
	RecentSearches []string
	// @SessionState
	Query string
	// @SessionState
	ConversationId string
	// @SessionState
	ConversationTitle string
	// @SessionState
	Author string
	// @SessionState
	From string
	// @SessionState
	To string
	// @SessionState
	HasAttachment bool
	// @SessionState
	IsEmail bool
	// @Needs
	queries *SearchQueryService
	// @Needs
	recent *SearchRecentService
	// @Needs
	shell                     *ShellService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnStart
func (search *SearchService) Start() {
	search.PaletteState = search.stateIdle()
	search.PageState = search.stateIdle()
}

func (search *SearchService) OpenPalette() {
	search.PaletteOpen = true
}

func (search *SearchService) ClosePalette() {
	search.PaletteOpen = false
}

func (search *SearchService) SearchPalette() {
	search.RecentSearches = search.recent.With(search.RecentSearches, search.PaletteQuery)
	search.askPalette(search.paletteFilter())
}

func (search *SearchService) SearchPaletteFor(query string) {
	search.PaletteQuery = query
	search.SearchPalette()
}

func (search *SearchService) ClearPaletteQuery() {
	search.PaletteQuery = ""
	search.askPalette(search.paletteFilter())
}

func (search *SearchService) ClearRecent() {
	search.RecentSearches = []string{}
}

func (search *SearchService) Open(row SearchHitRow) {
	opened := SearchHitOpenRequested{ConversationId: row.ConversationId, MessageId: row.MessageId}
	search.applicationEventPublisher.PublishEvent(opened, false)
	search.PaletteOpen = false
}

func (search *SearchService) SeeAll() {
	search.Query = search.PaletteQuery
	search.PaletteOpen = false
	search.shell.ShowPage(search.searchPage())
	search.Search()
}

func (search *SearchService) Search() {
	search.RecentSearches = search.recent.With(search.RecentSearches, search.Query)
	search.askPage(search.pageFilter())
}

func (search *SearchService) SearchWith(query string, author string, from string, to string) {
	search.Query = query
	search.Author = author
	search.From = from
	search.To = to
	search.Search()
}

func (search *SearchService) ClearQuery() {
	search.Query = ""
	search.Search()
}

func (search *SearchService) LoadMore() {
	search.searched.Cursor = search.NextCursor
	search.keepMore(search.queries.Find(search.searched))
}

// @OnEvent
func (search *SearchService) ScopeTo(requested SearchHitScopeRequested) {
	search.ConversationId = requested.ConversationId
	search.ConversationTitle = requested.Title
	search.Search()
}

func (search *SearchService) ClearScope() {
	search.ScopeTo(SearchHitScopeRequested{})
}

func (search *SearchService) ToggleAttachment() {
	search.HasAttachment = !search.HasAttachment
	search.Search()
}

func (search *SearchService) ToggleEmail() {
	search.IsEmail = !search.IsEmail
	search.Search()
}

func (search *SearchService) ClearFilters() {
	search.ConversationId = ""
	search.ConversationTitle = ""
	search.Author = ""
	search.From = ""
	search.To = ""
	search.HasAttachment = false
	search.IsEmail = false
	search.Search()
}

// @OnEvent
func (search *SearchService) DomainChanged(changed SessionDomainChanged) {
	search.clearWhenMoved(search.domainId, changed.DomainId)
	search.domainId = changed.DomainId
}

func (search *SearchService) clearWhenMoved(previous string, current string) {
	if previous == "" || previous == current {
		return
	}
	search.PaletteRows = []SearchHitRow{}
	search.PageRows = []SearchHitRow{}
	search.NextCursor = ""
	search.MoreState = ""
	search.PaletteState = search.stateIdle()
	search.PageState = search.stateIdle()
	search.ConversationId = ""
	search.ConversationTitle = ""
}

func (search *SearchService) paletteFilter() SearchHitFilter {
	return SearchHitFilter{Query: strings.TrimSpace(search.PaletteQuery), Limit: search.paletteLimit()}
}

func (search *SearchService) pageFilter() SearchHitFilter {
	return SearchHitFilter{
		Query:          strings.TrimSpace(search.Query),
		ConversationId: search.ConversationId,
		Author:         strings.TrimSpace(search.Author),
		From:           search.From,
		To:             search.To,
		HasAttachment:  search.HasAttachment,
		IsEmail:        search.IsEmail,
		Limit:          search.pageLimit(),
	}
}

func (search *SearchService) askPalette(filter SearchHitFilter) {
	search.PaletteRows = []SearchHitRow{}
	search.PaletteState = search.stateIdle()
	if filter.Query == "" {
		return
	}
	search.keepPalette(search.queries.Find(filter))
}

func (search *SearchService) askPage(filter SearchHitFilter) {
	search.searched = filter
	search.SearchedQuery = filter.Query
	search.PageRows = []SearchHitRow{}
	search.NextCursor = ""
	search.MoreState = ""
	search.PageState = search.stateIdle()
	if filter.Query == "" {
		return
	}
	search.keepPage(search.queries.Find(filter))
}

func (search *SearchService) keepPalette(answer SearchHitAnswer) {
	search.PaletteRows = answer.Rows
	search.PaletteState = answer.State
}

func (search *SearchService) keepPage(answer SearchHitAnswer) {
	search.PageRows = answer.Rows
	search.NextCursor = answer.NextCursor
	search.PageState = answer.State
}

func (search *SearchService) keepMore(answer SearchHitAnswer) {
	search.PageRows = append(search.PageRows, answer.Rows...)
	search.NextCursor = answer.NextCursor
	search.MoreState = answer.State
}

func (search *SearchService) searchPage() string {
	return "search"
}

func (search *SearchService) paletteLimit() string {
	return "5"
}

func (search *SearchService) pageLimit() string {
	return "20"
}

func (search *SearchService) stateIdle() string {
	return "idle"
}
