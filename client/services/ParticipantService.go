import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// ParticipantService fills the contact sidebar of the open message: the
// saved local contact, the opted-in public GitHub profile and related
// conversations from the same sender.
//
// @Service
type ParticipantService struct {
	// @Needs
	contactsClient *ContactsClient
	// @Needs
	searchClient *SearchClient
	// @Needs
	githubClient              *GithubClient
	applicationEventPublisher ApplicationEventPublisher

	SidebarOpen      bool
	SidebarLabel     string
	Loading          bool
	Error            string
	Address          string
	DisplayName      string
	Initials         string
	Contact          Contact
	HasContact       bool
	GithubStatus     string
	GithubProfile    GithubProfile
	HasGithubProfile bool
	Threads          []ParticipantThread
	VisibleThreads   []ParticipantThread
	HasMoreThreads   bool
	ThreadsExpanded  bool

	token string
}

// @OnStart
func (service *ParticipantService) Start() {
	service.reset("")
}

// @OnEvent({"replay":true})
func (service *ParticipantService) HearSession(changed SessionChanged) {
	service.token = changed.Token
}

// HearStorage restores whether the sidebar was open on this device.
// @OnEvent({"replay":true})
func (service *ParticipantService) HearStorage(stored StoredValues) {
	service.SidebarOpen = stored.Local[service.sidebarKey()] == service.opened()
	service.refreshLabels()
}

// HearMessage shows the sender of the message the reader opened.
// @OnEvent({"replay":true})
func (service *ParticipantService) HearMessage(opened OpenMessageChanged) {
	FlowUtils{}.When(opened.ParticipantAddress != service.Address, func() {
		service.reset(opened.ParticipantAddress)
		service.load()
	})()
	service.applicationEventPublisher.PublishEvent(ViewSettled{Source: "participant"}, false)
}

// @OnEvent
func (service *ParticipantService) Settled(_ ViewSettled) {}

func (service *ParticipantService) Toggle() {
	service.SidebarOpen = !service.SidebarOpen
	service.applicationEventPublisher.PublishEvent(StorageWritten{Area: "local", Values: map[string]string{service.sidebarKey(): TextUtils{}.Choose(service.SidebarOpen, service.opened(), "closed")}, Removed: []string{}}, false)
	service.refreshLabels()
	service.load()
}

func (ParticipantService) sidebarKey() string { return "caduceus-participant-sidebar" }

func (ParticipantService) opened() string { return "open" }

func (service *ParticipantService) Close() {
	service.close(service.SidebarOpen)
}

func (service *ParticipantService) ToggleThreads() {
	service.ThreadsExpanded = !service.ThreadsExpanded
	service.refreshLabels()
}

// OpenThread opens a related message on its own page.
func (service *ParticipantService) OpenThread(messageID string) {
	service.applicationEventPublisher.PublishEvent(ComposeUtils{}.Message(messageID), false)
}

func (ParticipantService) disabled() string { return "disabled" }

func (service *ParticipantService) reset(address string) {
	service.Address = address
	service.Contact = Contact{}
	service.HasContact = false
	service.GithubStatus = service.disabled()
	service.GithubProfile = GithubProfile{Repositories: []GithubRepository{}}
	service.HasGithubProfile = false
	service.Threads = []ParticipantThread{}
	service.ThreadsExpanded = false
	service.Error = ""
	service.refreshLabels()
}

func (service *ParticipantService) close(visible bool) {
	if visible {
		service.Toggle()
	}
}

func (service *ParticipantService) load() {
	FlowUtils{}.When(LogicUtils{}.All(service.Address != "", service.SidebarOpen), func() { service.fetch(service.authorization()) })()
}

func (service *ParticipantService) authorization() string {
	return "Bearer " + service.token
}

func (service *ParticipantService) fetch(authorization string) {
	service.Loading = true
	service.Error = ""
	contacts, contactsFailure := service.contactsClient.List(authorization, service.Address, "", "")
	related, relatedFailure := service.searchClient.Search(authorization, "from:"+service.Address, service.searchLimit(), "", "")
	github, githubFailure := service.githubClient.Profile(authorization, service.Address)
	service.Contact, service.HasContact = service.matchingContact(contacts)
	service.showGithub(github, githubFailure == nil)
	service.Threads = service.threads(related)
	service.offline(slices.ContainsFunc([]error{contactsFailure, relatedFailure, githubFailure}, func(failure error) bool { return LogicUtils{}.All(failure != nil, !HTTPUtils{}.Refused(failure)) }))
	service.Loading = false
	service.refreshLabels()
}

func (service *ParticipantService) matchingContact(contacts []Contact) (Contact, bool) {
	index := slices.IndexFunc(contacts, func(contact Contact) bool { return service.hasAddress(contact.Emails) })
	return service.contactAt(contacts, index)
}

func (service *ParticipantService) hasAddress(emails []ContactValue) bool {
	return slices.ContainsFunc(emails, func(entry ContactValue) bool { return strings.ToLower(entry.Value) == service.Address })
}

func (service *ParticipantService) contactAt(contacts []Contact, index int) (Contact, bool) {
	if index < 0 {
		return Contact{}, false
	}
	return contacts[index], true
}

func (ParticipantService) found() string { return "found" }

func (service *ParticipantService) showGithub(lookup GithubLookup, loaded bool) {
	service.GithubStatus = service.githubStatus(lookup.Status, loaded)
	service.GithubProfile, service.HasGithubProfile = service.profile(lookup.Profile, service.GithubStatus == service.found())
}

func (ParticipantService) unavailable() string { return "unavailable" }

func (service *ParticipantService) githubStatus(status string, loaded bool) string {
	known := slices.Contains([]string{service.disabled(), service.found(), "not-found", service.unavailable()}, status)
	if (LogicUtils{}).All(loaded, known) {
		return status
	}
	return service.unavailable()
}

func (service *ParticipantService) profile(profile *GithubProfile, found bool) (GithubProfile, bool) {
	if profile == nil {
		return GithubProfile{Repositories: []GithubRepository{}}, false
	}
	return *profile, found
}

func (ParticipantService) maximumThreads() int { return 25 }

func (service *ParticipantService) threads(related []Email) []ParticipantThread {
	fromSender := slices.DeleteFunc(slices.Clone(related), func(message Email) bool { return strings.ToLower(message.Sender) != service.Address })
	sort.SliceStable(fromSender, func(left int, right int) bool { return fromSender[left].ReceivedAt > fromSender[right].ReceivedAt })
	unique := service.unique(fromSender)
	return service.summaries(unique[:min(len(unique), service.maximumThreads())])
}

func (service *ParticipantService) unique(messages []Email) []Email {
	seen := map[string]bool{}
	kept := []Email{}
	for _, message := range messages {
		kept = append(kept, service.firstOf(message, seen)...)
	}
	return kept
}

func (service *ParticipantService) firstOf(message Email, seen map[string]bool) []Email {
	key := TextUtils{}.FirstOf(TextUtils{}.FirstOf(TextUtils{}.FirstOf(message.ThreadID, message.MessageID), message.Subject), message.ID)
	if (LogicUtils{}).Any(key == "", seen[key]) {
		return []Email{}
	}
	seen[key] = true
	return []Email{message}
}

func (ParticipantService) space() string { return " " }

func (ParticipantService) maximumSnippet() int { return 120 }

// searchLimit is the server's largest search answer, which the old client got by sending no limit.
func (ParticipantService) searchLimit() int { return 1000 }

func (service *ParticipantService) summaries(messages []Email) []ParticipantThread {
	whitespace := regexp.MustCompile(`\s+`)
	threads := []ParticipantThread{}
	for _, message := range messages {
		snippet := []rune(strings.TrimSpace(whitespace.ReplaceAllString(TextUtils{}.FirstOf(message.Snippet, message.Body), service.space())))
		threads = append(threads, ParticipantThread{Id: message.ID, Subject: TextUtils{}.FirstOf(message.Subject, "(No subject)"), Snippet: string(snippet[:min(len(snippet), service.maximumSnippet())]), ReceivedAt: message.ReceivedAt, ReceivedLabel: DateLabelUtils{}.Stamp(message.ReceivedAt, time.Now())})
	}
	return threads
}

func (service *ParticipantService) offline(failed bool) {
	if failed {
		service.GithubStatus = service.unavailable()
		service.GithubProfile = GithubProfile{Repositories: []GithubRepository{}}
		service.HasGithubProfile = false
		service.Error = "Contact context is unavailable."
	}
}

func (ParticipantService) visibleThreadCount() int { return 3 }

func (service *ParticipantService) refreshLabels() {
	service.SidebarLabel = TextUtils{}.Choose(service.SidebarOpen, "Hide contact sidebar", "Show contact sidebar")
	service.DisplayName = TextUtils{}.FirstOf(TextUtils{}.FirstOf(service.Contact.Name, strings.TrimSpace(service.Contact.GivenName+service.space()+service.Contact.FamilyName)), service.Address)
	service.Initials = AddressUtils{}.Initials(TextUtils{}.FirstOf(service.DisplayName, service.Address))
	service.HasMoreThreads = len(service.Threads) > service.visibleThreadCount()
	service.VisibleThreads = service.visibleThreads(service.ThreadsExpanded)
}

func (service *ParticipantService) visibleThreads(expanded bool) []ParticipantThread {
	if expanded {
		return service.Threads
	}
	return service.Threads[:min(len(service.Threads), service.visibleThreadCount())]
}
