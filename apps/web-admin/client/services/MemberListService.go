import (
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
	HasMore       bool
	// @Needs
	session *AdminSessionService
	// @Needs
	directoryClient *DirectoryClient
	// @Needs
	problems *AdminProblemService
}

// @OnEvent({"condition":"value.view == 'members'"})
// @InBackground
func (list *MemberListService) Opened(opened AdminViewOpened) {
	list.DomainId = opened.DomainId
	list.Query = ""
	list.load()
}

func (list *MemberListService) Search() {
	list.load()
}

func (list *MemberListService) ClearSearch() {
	list.Query = ""
	list.load()
}

func (list *MemberListService) Retry() {
	list.load()
}

func (list *MemberListService) load() {
	list.State = "loading"
	list.SearchedQuery = strings.TrimSpace(list.Query)
	page, failure := list.directoryClient.Members(list.session.Authorization(), list.SearchedQuery)
	problem := list.problems.Of(failure)
	list.Rows = list.rowsOf(page.Items)
	list.HasMore = page.NextCursor != ""
	list.Heading = list.countText(len(list.Rows)) + list.shownSuffix(list.HasMore)
	list.State = list.emptyState(list.SearchedQuery)
	list.State = list.filledState(len(list.Rows), list.State)
	list.State = list.failedState(problem, list.State)
	list.State = list.forbiddenState(problem, list.State)
}

func (list *MemberListService) rowsOf(items []MemberSummary) []MemberRow {
	rows := []MemberRow{}
	for _, item := range items {
		rows = append(rows, MemberRow{MemberId: item.MemberId, Name: item.DisplayName, Address: item.PrimaryAddress, Initials: list.initialsOf(item.DisplayName)})
	}
	return rows
}

func (list *MemberListService) initialsOf(name string) string {
	words := append(strings.Fields(name), "", "")
	return strings.ToUpper(list.firstLetter(words[0]) + list.firstLetter(words[1]))
}

func (list *MemberListService) firstLetter(word string) string {
	letters := []rune(word)
	return string(letters[:min(1, len(letters))])
}

func (list *MemberListService) countText(count int) string {
	if count == 1 {
		return "1 member"
	}
	return strconv.Itoa(count) + " members"
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
	return "ready"
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
