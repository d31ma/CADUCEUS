import (
	"slices"
	"strconv"
	"strings"
)

// @Service
type PeopleListService struct {
	Query         string
	StateFilter   string
	SearchedQuery string
	State         string
	Rows          []PersonRow
	Heading       string
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

// @OnEvent({"condition":"value.view == 'people'","replay":true})
// @InBackground
func (people *PeopleListService) Opened(opened AdminViewOpened) {
	people.Query = ""
	people.StateFilter = ""
	people.load()
	people.applicationEventPublisher.PublishEvent(AdminViewLoaded{View: opened.View, Message: people.Heading}, false)
}

// @OnEvent
// @InBackground
func (people *PeopleListService) Changed(changed PeopleChanged) {
	people.load()
}

func (people *PeopleListService) SearchWith(query string, stateFilter string) {
	people.Query = query
	people.StateFilter = people.chosenFilter(stateFilter)
	people.load()
}

func (people *PeopleListService) ClearSearch() {
	people.Query = ""
	people.StateFilter = ""
	people.load()
}

func (people *PeopleListService) Retry() {
	people.load()
}

func (people *PeopleListService) load() {
	people.State = "loading"
	people.SearchedQuery = strings.TrimSpace(people.Query)
	page, failure := people.directoryClient.People(people.session.Authorization(), people.SearchedQuery)
	problem := people.problems.Of(failure)
	people.Rows = people.rowsOf(slices.DeleteFunc(slices.Clone(page.Items), people.filteredOut))
	people.HasMore = page.NextCursor != ""
	people.Heading = people.countText(len(people.Rows)) + people.shownSuffix(people.HasMore)
	people.State = people.emptyState(people.SearchedQuery + people.StateFilter)
	people.State = people.filledState(len(people.Rows), people.State)
	people.State = people.failedState(problem, people.State)
	people.State = people.forbiddenState(problem, people.State)
}

func (people *PeopleListService) filteredOut(person PersonSummary) bool {
	return people.StateFilter != "" && person.State != people.StateFilter
}

func (people *PeopleListService) chosenFilter(stateFilter string) string {
	if stateFilter == "all" {
		return ""
	}
	return stateFilter
}

func (people *PeopleListService) rowsOf(items []PersonSummary) []PersonRow {
	rows := []PersonRow{}
	for _, item := range items {
		rows = append(rows, people.rowOf(item))
	}
	return rows
}

func (people *PeopleListService) rowOf(item PersonSummary) PersonRow {
	return PersonRow{
		PersonId:       item.PersonId,
		Name:           item.DisplayName,
		Username:       item.Username,
		Initials:       people.initials.Of(item.DisplayName + " " + item.Username),
		DomainIds:      item.DomainIds,
		State:          item.State,
		StateText:      people.stateText(item.State),
		StateColor:     map[string]string{people.active(): "success", people.disabled(): "warning"}[item.State],
		NoDomains:      len(item.DomainIds) == 0,
		Deleted:        item.State == people.deleted(),
		Disabled:       item.State == people.disabled(),
		AccountManaged: item.State != people.deleted(),
	}
}

func (people *PeopleListService) stateText(state string) string {
	text, known := map[string]string{people.active(): "Active", people.disabled(): "Turned off", people.deleted(): "Deleted"}[state]
	if !known {
		return state
	}
	return text
}

func (people *PeopleListService) countText(count int) string {
	if count == 1 {
		return "1 person"
	}
	return strconv.Itoa(count) + " people"
}

func (people *PeopleListService) shownSuffix(hasMore bool) string {
	if !hasMore {
		return ""
	}
	return " shown"
}

func (people *PeopleListService) emptyState(filters string) string {
	if filters == "" {
		return "empty"
	}
	return "noResults"
}

func (people *PeopleListService) filledState(count int, state string) string {
	if count == 0 {
		return state
	}
	return "ready"
}

func (people *PeopleListService) failedState(problem string, state string) string {
	if problem == "" {
		return state
	}
	return "failed"
}

func (people *PeopleListService) forbiddenState(problem string, state string) string {
	if problem != people.forbidden() {
		return state
	}
	return people.forbidden()
}

func (people *PeopleListService) forbidden() string {
	return "forbidden"
}

func (people *PeopleListService) active() string {
	return "active"
}

func (people *PeopleListService) disabled() string {
	return "disabled"
}

func (people *PeopleListService) deleted() string {
	return "deleted"
}
