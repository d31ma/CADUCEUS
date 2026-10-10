import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// ThreadSearchService is the mailbox search box: the typed query with its
// highlighted field names, and the listbox of suggestions (field names and
// values, folders, contacts and subjects). Submitting asks InboxService to
// search with a SearchSubmitted event. MailboxBrowserService sends the keys
// and the pause in typing that need the browser event.
//
// @Service
type ThreadSearchService struct {
	// @Needs
	contactsClient *ContactsClient
	// @Needs
	searchClient              *SearchClient
	applicationEventPublisher ApplicationEventPublisher

	Draft            string
	Placeholder      string
	Segments         []SearchSegment
	Suggestions      []SearchSuggestionRow
	Visible          bool
	Expanded         string
	ActiveDescendant *string
	StatusText       string
	LayerStyle       string

	caret         int
	focused       bool
	selectedIndex int
	request       int
	choices       []SearchSuggestion
	scope         MailboxScopeChanged
	accounts      []CategoryAccount
	folders       map[string][]ExternalFolder
	authorization string
	owner         string
}

// @OnStart
func (search *ThreadSearchService) Start() {
	search.selectedIndex = -1
	search.derive()
}

// @OnEvent({"replay":true})
func (search *ThreadSearchService) SessionChanged(session SessionChanged) {
	search.authorization = "Bearer " + session.Token
	search.folders = session.ExternalFolders
	search.resetWhenOwnerChanged(strings.ToLower(session.Email))
	search.derive()
}

// @OnEvent({"replay":true})
func (search *ThreadSearchService) MailboxScopeChanged(scope MailboxScopeChanged) {
	search.scope = scope
	search.derive()
}

// @OnEvent({"replay":true})
func (search *ThreadSearchService) CategoryCatalogChanged(catalog CategoryCatalogChanged) {
	search.accounts = catalog.Accounts
}

// Input keeps the typed query and offers the suggestions known without the
// server at once; the server-backed ones follow when typing pauses.
func (search *ThreadSearchService) Input(value string, caret int, scrolled float64) {
	search.request++
	search.Draft = value
	search.focused = true
	search.caret = caret
	search.choices = ThreadSearchUtils{}.StaticSuggestions(value, caret)
	search.selectedIndex = min(len(search.choices), 1) - 1
	search.LayerStyle = search.layerStyle(scrolled)
	search.derive()
}

// Focus seeds an empty box with the current folder and filter as search
// terms, then opens the suggestions.
func (search *ThreadSearchService) Focus(value string, caret int) {
	search.focused = true
	seeded := cmp.Or(value, search.seed())
	search.Draft = seeded
	search.caret = map[bool]int{true: len([]rune(seeded)), false: caret}[value == ""]
	search.refresh(seeded, search.caret)
	search.derive()
}

// Scrolled keeps the highlighted tokens under a query that scrolls sideways.
func (search *ThreadSearchService) Scrolled(scrolled float64) {
	search.LayerStyle = search.layerStyle(scrolled)
}

// Submit searches the list for the query in the box.
func (search *ThreadSearchService) Submit() {
	search.focused = false
	search.choices = []SearchSuggestion{}
	search.selectedIndex = -1
	search.applicationEventPublisher.PublishEvent(SearchSubmitted{Query: search.Draft}, false)
	search.derive()
}

// Clear empties the box and shows the folder without filters again.
func (search *ThreadSearchService) Clear() {
	search.request++
	search.choices = []SearchSuggestion{}
	search.selectedIndex = -1
	search.Draft = ""
	search.applicationEventPublisher.PublishEvent(SearchSubmitted{ResetStatus: true}, false)
	search.derive()
}

// @OnEvent
func (search *ThreadSearchService) SearchKeyPressed(pressed SearchKeyPressed) {
	search.runKey(pressed)
	search.derive()
	search.settle()
}

// @OnEvent
func (search *ThreadSearchService) SearchSuggestionChosen(chosen SearchSuggestionChosen) {
	search.choose(append(slices.Clone(search.choices), SearchSuggestion{})[max(0, min(chosen.Index, len(search.choices)))], chosen.Value, chosen.Caret)
	search.derive()
	search.settle()
}

// @OnEvent
func (search *ThreadSearchService) SearchSuggestionsWanted(wanted SearchSuggestionsWanted) {
	search.refresh(wanted.Value, wanted.Caret)
	search.derive()
	search.settle()
}

// @OnEvent
func (search *ThreadSearchService) SearchBlurred(_ SearchBlurred) {
	search.focused = false
	search.selectedIndex = -1
	search.derive()
}

func (search *ThreadSearchService) resetWhenOwnerChanged(owner string) {
	if owner != search.owner {
		search.owner = owner
		search.Draft = ""
		search.choices = []SearchSuggestion{}
		search.focused = false
		search.selectedIndex = -1
		search.caret = 0
		search.request = 0
	}
}

func (search *ThreadSearchService) runKey(pressed SearchKeyPressed) {
	if action, found := search.keyActions()[pressed.Key]; found {
		action(pressed)
	}
}

func (search *ThreadSearchService) keyActions() map[string]func(SearchKeyPressed) {
	return map[string]func(SearchKeyPressed){
		"ArrowDown": func(pressed SearchKeyPressed) { search.step(pressed, 1) },
		"ArrowUp":   func(pressed SearchKeyPressed) { search.step(pressed, -1) },
		"Escape":    search.dismiss,
		"Enter":     search.enter,
	}
}

func (search *ThreadSearchService) step(pressed SearchKeyPressed, direction int) {
	count := max(len(search.choices), 1)
	search.focused = true
	search.selectedIndex = (search.selectedIndex + direction + count) % count
	search.Draft = pressed.Value
}

func (search *ThreadSearchService) dismiss(pressed SearchKeyPressed) {
	search.choices = []SearchSuggestion{}
	search.selectedIndex = -1
	search.Draft = pressed.Value
}

func (search *ThreadSearchService) enter(pressed SearchKeyPressed) {
	if search.selectedIndex >= 0 && search.selectedIndex < len(search.choices) {
		search.choose(search.choices[search.selectedIndex], pressed.Value, pressed.Caret)
		return
	}
	search.Draft = pressed.Value
	search.Submit()
}

func (search *ThreadSearchService) choose(suggestion SearchSuggestion, value string, caret int) {
	applied := ThreadSearchUtils{}.Apply(value, caret, suggestion)
	search.Draft = applied.Query
	search.caret = len([]rune(applied.Query))
	search.choices = []SearchSuggestion{}
	search.selectedIndex = -1
	search.afterChoice(applied)
}

func (search *ThreadSearchService) afterChoice(applied AppliedSearch) {
	if applied.Submit {
		search.Submit()
		return
	}
	search.refresh(applied.Query, search.caret)
}

// refresh builds the local vocabulary first, then enriches it with the
// owner's contacts, folders and subjects. A newer edit wins over an answer
// that arrives late.
func (search *ThreadSearchService) refresh(query string, caret int) {
	search.request++
	request := search.request
	current := ThreadSearchUtils{}.Current(query, caret)
	term := strings.TrimSpace(current.Term)
	lookup := cmp.Or(term, map[bool]string{true: strings.TrimSpace(query)}[current.Token == ""])
	categories := search.categoryChoices(search.categoryValues(search.categoriesOf(search.scopedAccounts())), term)
	folders := search.folderChoices(search.folderValues(), term)
	contacts := search.contactsWhen(search.wantsContacts(current, query), lookup)
	subjects := search.subjectsWhen(search.wantsSubjects(current, query), lookup, search.scopeDomain())
	dynamic := slices.Concat(map[bool][]SearchSuggestion{true: slices.Concat(categories, folders)}[current.Token == "in"], search.contactChoices(contacts, current), search.subjectChoices(subjects))
	search.applyWhenCurrent(request, query, caret, slices.Concat(ThreadSearchUtils{}.StaticSuggestions(query, caret), dynamic))
}

func (search *ThreadSearchService) scopedAccounts() []CategoryAccount {
	if search.scope.Account != "" {
		return []CategoryAccount{}
	} else if search.everyMailbox() {
		return search.accounts
	}
	return slices.DeleteFunc(slices.Clone(search.accounts), func(account CategoryAccount) bool { return account.Domain != search.scope.Domain })
}

func (search *ThreadSearchService) categoriesOf(accounts []CategoryAccount) []Category {
	categories := []Category{}
	for _, account := range accounts {
		categories = append(categories, account.Categories...)
	}
	return categories
}

func (search *ThreadSearchService) categoryValues(categories []Category) []SearchSuggestion {
	values := []SearchSuggestion{}
	for _, category := range categories {
		value := strings.TrimSpace(cmp.Or(category.Role, category.Path))
		values = append(values, SearchSuggestion{Token: search.inField(), Term: value, Description: cmp.Or(category.Path, value), Submit: true, Kind: search.categoryKind()})
	}
	return values
}

func (ThreadSearchService) inField() string { return "in" }

func (ThreadSearchService) categoryKind() string { return "category" }

func (search *ThreadSearchService) categoryChoices(values []SearchSuggestion, term string) []SearchSuggestion {
	seen := map[string]bool{}
	unique := slices.DeleteFunc(values, func(value SearchSuggestion) bool {
		lower := strings.ToLower(value.Term)
		repeated := seen[lower]
		seen[lower] = true
		return repeated
	})
	return search.wrapped(slices.DeleteFunc(unique, search.unmatched(term)))
}

func (search *ThreadSearchService) folderValues() []SearchSuggestion {
	choices := []SearchSuggestion{}
	for _, folder := range search.folders[search.scope.Account] {
		value := strings.TrimSpace(cmp.Or(folder.Role, folder.Name))
		choices = append(choices, SearchSuggestion{Token: search.inField(), Term: value, Description: cmp.Or(folder.Name, value), Submit: true, Kind: search.categoryKind()})
	}
	return choices
}

func (search *ThreadSearchService) folderChoices(choices []SearchSuggestion, term string) []SearchSuggestion {
	return search.wrapped(slices.DeleteFunc(choices, search.unmatched(term)))
}

func (ThreadSearchService) unmatched(term string) func(SearchSuggestion) bool {
	return func(choice SearchSuggestion) bool {
		if choice.Term == "" || (term != "" && !strings.Contains(strings.ToLower(choice.Term), term)) {
			return true
		}
		return false
	}
}

func (search *ThreadSearchService) wrapped(values []SearchSuggestion) []SearchSuggestion {
	wrapped := []SearchSuggestion{}
	for _, value := range values {
		value.Term = map[bool]string{true: ThreadSearchUtils{}.Wrap(value.Term), false: value.Term}[strings.ContainsAny(value.Term, search.blanks())]
		wrapped = append(wrapped, value)
	}
	return wrapped
}

func (ThreadSearchService) blanks() string { return " \t\n" }

func (search *ThreadSearchService) wantsContacts(current SearchToken, query string) bool {
	if current.Token == "from" || current.Token == search.toField() || (current.Token == "" && len([]rune(strings.TrimSpace(query))) >= search.contactLength()) {
		return true
	}
	return false
}

func (ThreadSearchService) toField() string { return "to" }

func (ThreadSearchService) contactLength() int { return 2 }

func (search *ThreadSearchService) wantsSubjects(current SearchToken, query string) bool {
	if current.Token == search.subjectField() || (current.Token == "" && len([]rune(strings.TrimSpace(query))) >= search.subjectLength()) {
		return true
	}
	return false
}

func (ThreadSearchService) subjectField() string { return "subject" }

func (ThreadSearchService) subjectLength() int { return 3 }

func (search *ThreadSearchService) contactsWhen(wanted bool, lookup string) []ContactSuggestion {
	if !wanted {
		return []ContactSuggestion{}
	}
	found, _ := search.contactsClient.Suggestions(search.authorization, lookup)
	return found
}

func (search *ThreadSearchService) scopeDomain() string {
	if search.scope.Account != "" || search.scope.Domain == search.everyName() {
		return ""
	}
	return search.scope.Domain
}

func (ThreadSearchService) everyName() string { return "all" }

func (search *ThreadSearchService) everyMailbox() bool {
	return slices.Contains([]string{search.everyName(), ""}, search.scope.Domain)
}

func (search *ThreadSearchService) subjectsWhen(wanted bool, lookup string, domain string) []Email {
	if !wanted {
		return []Email{}
	}
	found, _ := search.searchClient.Search(search.authorization, search.subjectField()+search.colon()+ThreadSearchUtils{}.Wrap(lookup), search.subjectLimit(), domain, search.scope.Account)
	return found
}

func (ThreadSearchService) colon() string { return ":" }

func (ThreadSearchService) subjectLimit() int { return 12 }

func (search *ThreadSearchService) contactChoices(contacts []ContactSuggestion, current SearchToken) []SearchSuggestion {
	choices := []SearchSuggestion{}
	field := map[bool]string{true: search.toField(), false: "from"}[current.Token == search.toField()]
	for _, contact := range slices.DeleteFunc(slices.Clone(contacts), func(contact ContactSuggestion) bool { return contact.Email == "" }) {
		described := map[bool]string{true: contact.Email, false: contact.Name + " · " + contact.Email}[contact.Name == ""]
		choices = append(choices, SearchSuggestion{Token: field, Term: ThreadSearchUtils{}.Wrap(contact.Email), Description: described, Submit: true, Kind: "contact"})
	}
	return choices
}

func (search *ThreadSearchService) subjectChoices(messages []Email) []SearchSuggestion {
	seen := map[string]bool{"": true}
	choices := []SearchSuggestion{}
	for _, message := range slices.DeleteFunc(slices.Clone(messages), func(message Email) bool {
		subject := strings.ToLower(strings.TrimSpace(message.Subject))
		repeated := seen[subject]
		seen[subject] = true
		return repeated
	}) {
		subject := strings.TrimSpace(message.Subject)
		choices = append(choices, SearchSuggestion{Token: search.subjectField(), Term: ThreadSearchUtils{}.Wrap(subject), Description: subject, Submit: true, Kind: search.subjectField()})
	}
	return choices
}

func (search *ThreadSearchService) applyWhenCurrent(request int, query string, caret int, suggestions []SearchSuggestion) {
	if request != search.request {
		return
	}
	seen := map[string]bool{}
	unique := slices.DeleteFunc(suggestions, func(suggestion SearchSuggestion) bool {
		key := strings.ToLower(suggestion.Token + "\x00" + suggestion.Term)
		repeated := seen[key]
		seen[key] = true
		return repeated
	})
	search.choices = unique[:min(len(unique), search.mostSuggestions())]
	search.selectedIndex = min(len(search.choices), 1) - 1
	search.Draft = query
	search.caret = caret
}

func (ThreadSearchService) mostSuggestions() int { return 12 }

func (search *ThreadSearchService) derive() {
	search.Visible = search.visible()
	search.Expanded = map[bool]string{true: "true", false: "false"}[search.Visible]
	search.Segments = ThreadSearchUtils{}.Segments(search.Draft)
	search.Suggestions = search.rows()
	search.ActiveDescendant = search.activeDescendant()
	search.StatusText = map[bool]string{true: fmt.Sprintf("%d search suggestions available", len(search.choices))}[search.Visible]
	search.Placeholder = search.placeholder()
}

func (search *ThreadSearchService) visible() bool {
	if search.focused && len(search.choices) > 0 {
		return true
	}
	return false
}

func (search *ThreadSearchService) rows() []SearchSuggestionRow {
	rows := []SearchSuggestionRow{}
	for index, suggestion := range search.choices {
		selected := index == search.selectedIndex
		rows = append(rows, SearchSuggestionRow{
			ID: search.optionID(index), Index: index, Description: suggestion.Description,
			Title:     map[bool]string{true: suggestion.Term, false: suggestion.Token + search.colon() + suggestion.Term}[suggestion.Token == ""],
			ClassName: map[bool]string{true: "search-suggestion active", false: "search-suggestion"}[selected],
			Selected:  map[bool]string{true: "true", false: "false"}[selected],
		})
	}
	return rows
}

func (search *ThreadSearchService) activeDescendant() *string {
	if !search.Visible || search.selectedIndex < 0 || search.selectedIndex >= len(search.choices) {
		return nil
	}
	id := search.optionID(search.selectedIndex)
	return &id
}

func (search *ThreadSearchService) optionID(index int) string {
	return fmt.Sprintf("thread-search-option-%d-%d", search.request, index)
}

func (search *ThreadSearchService) placeholder() string {
	if search.scope.Account != "" {
		return search.searchWord() + search.scope.AccountLabel
	} else if search.everyMailbox() {
		return "Search all mailboxes"
	}
	return search.searchWord() + search.scope.DomainLabel
}

func (ThreadSearchService) searchWord() string { return "Search " }

// seed is the current perspective as Mailspring search terms, such as
// "is:unread in:archive ".
func (search *ThreadSearchService) seed() string {
	terms := []string{map[string]string{"unread": "is:unread", "starred": "is:starred"}[search.scope.StatusFilter], search.folderTerm(search.scope.Folder)}
	space := " "
	joined := strings.Join(slices.DeleteFunc(terms, func(term string) bool { return term == "" }), space)
	return map[bool]string{true: "", false: joined + space}[joined == ""]
}

func (search *ThreadSearchService) folderTerm(folder string) string {
	if folder == "" || folder == search.everyName() || folder == "inbox" {
		return ""
	}
	return search.inField() + search.colon() + map[bool]string{true: ThreadSearchUtils{}.Wrap(folder), false: folder}[strings.ContainsAny(folder, search.blanks())]
}

func (search *ThreadSearchService) layerStyle(scrolled float64) string {
	return fmt.Sprintf("transform:translateX(%vpx)", -scrolled)
}

func (search *ThreadSearchService) settle() {
	search.applicationEventPublisher.PublishEvent(ViewSettled{Source: "thread-search"}, false)
}
