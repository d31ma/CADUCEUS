import (
	"slices"
	"strconv"
	"strings"
)

// @Service
type MemberListService struct {
	DomainId      string
	Query         string
	SearchedQuery string
	State         string
	Rows          []MemberRow
	Heading       string
	AdminCount    string
	HasMore       bool
	// @Needs
	session *AdminSessionService
	// @Needs
	directoryClient *DirectoryClient
	// @Needs
	problems *AdminProblemService
	// @Needs
	initials                  *InitialsService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnEvent({"condition":"value.view == 'members'","replay":true})
// @InBackground
func (list *MemberListService) Opened(opened AdminViewOpened) {
	list.DomainId = opened.DomainId
	list.Query = ""
	list.load()
	list.applicationEventPublisher.PublishEvent(AdminViewLoaded{View: opened.View, Message: list.Heading}, false)
}

// @OnEvent
// @InBackground
func (list *MemberListService) Changed(changed PeopleChanged) {
	list.load()
}

func (list *MemberListService) SearchWith(query string) {
	list.Query = query
	list.load()
}

func (list *MemberListService) ClearSearch() {
	list.Query = ""
	list.load()
}

func (list *MemberListService) Retry() {
	list.load()
}

func (list *MemberListService) NameOf(username string) string {
	return append(slices.Clone(list.Rows), MemberRow{})[list.foundIndex(list.indexOf(username), len(list.Rows))].Name
}

func (list *MemberListService) indexOf(username string) int {
	return slices.IndexFunc(list.Rows, func(row MemberRow) bool { return row.Username == username })
}

func (list *MemberListService) Listed() bool {
	return map[string]bool{list.ready(): true, list.onlyYou(): true}[list.State]
}

func (list *MemberListService) load() {
	list.State = "loading"
	list.SearchedQuery = strings.TrimSpace(list.Query)
	page, failure := list.directoryClient.Members(list.session.Authorization(), list.SearchedQuery)
	problem := list.problems.Of(failure)
	list.Rows = list.rowsOf(page.Items)
	list.HasMore = page.NextCursor != ""
	list.Heading = list.countText(len(list.Rows), "member", "members") + list.shownSuffix(list.HasMore)
	list.AdminCount = list.countText(list.admins(), "admin", "admins")
	list.State = list.emptyState(list.SearchedQuery)
	list.State = list.filledState(len(list.Rows), list.State)
	list.State = list.aloneState(list.aloneKey(), list.State)
	list.State = list.failedState(problem, list.State)
	list.State = list.forbiddenState(problem, list.State)
}

func (list *MemberListService) rowsOf(items []MemberSummary) []MemberRow {
	rows := []MemberRow{}
	for _, item := range items {
		rows = append(rows, list.rowOf(item))
	}
	return rows
}

func (list *MemberListService) rowOf(item MemberSummary) MemberRow {
	return MemberRow{
		MemberId:       item.MemberId,
		Name:           item.DisplayName,
		Username:       item.Username,
		Address:        item.PrimaryAddress,
		Initials:       list.initials.Of(item.DisplayName),
		Admin:          item.Admin,
		AccountManaged: item.AccountManaged,
		You:            item.Username != "" && item.Username == list.session.Username,
	}
}

func (list *MemberListService) admins() int {
	count := 0
	for _, row := range list.Rows {
		count += map[bool]int{true: 1, false: 0}[row.Admin]
	}
	return count
}

func (list *MemberListService) aloneKey() string {
	return strings.Join([]string{list.SearchedQuery, strconv.Itoa(len(list.Rows)), strconv.FormatBool(list.firstIsYou())}, "/")
}

func (list *MemberListService) firstIsYou() bool {
	return append(slices.Clone(list.Rows), MemberRow{})[0].You
}

func (list *MemberListService) foundIndex(index int, missing int) int {
	if index < 0 {
		return missing
	}
	return index
}

func (list *MemberListService) countText(count int, one string, many string) string {
	if count == 1 {
		return "1 " + one
	}
	return strconv.Itoa(count) + " " + many
}

func (list *MemberListService) shownSuffix(hasMore bool) string {
	if !hasMore {
		return ""
	}
	return " shown"
}

func (list *MemberListService) emptyState(query string) string {
	if query == "" {
		return "empty"
	}
	return "noResults"
}

func (list *MemberListService) filledState(count int, state string) string {
	if count == 0 {
		return state
	}
	return list.ready()
}

func (list *MemberListService) ready() string {
	return "ready"
}

func (list *MemberListService) onlyYou() string {
	return "onlyYou"
}

func (list *MemberListService) aloneState(key string, state string) string {
	if key != "/1/true" {
		return state
	}
	return list.onlyYou()
}

func (list *MemberListService) failedState(problem string, state string) string {
	if problem == "" {
		return state
	}
	return "failed"
}

func (list *MemberListService) forbiddenState(problem string, state string) string {
	if problem != list.forbidden() {
		return state
	}
	return list.forbidden()
}

func (list *MemberListService) forbidden() string {
	return "forbidden"
}
