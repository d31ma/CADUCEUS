import (
	"regexp"
	"slices"
	"sort"
	"strings"
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
	Contact          ParticipantContact
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
func (service *ParticipantService) SessionChanged(changed SessionChanged) {
	service.token = changed.Token
}

// @OnEvent({"replay":true})
func (service *ParticipantService) ReaderOpened(opened ReaderOpened) {
	service.SidebarOpen = opened.ParticipantSidebar
	service.refreshLabels()
}

// @OnEvent({"replay":true})
func (service *ParticipantService) MessageOpened(opened OpenMessageChanged) error {
	service.reset(opened.ParticipantAddress)
	service.load()
	service.applicationEventPublisher.PublishEvent(ReaderChanged{Reason: "participant"}, false)
	return nil
}

func (service *ParticipantService) Toggle() {
	service.SidebarOpen = !service.SidebarOpen
	service.applicationEventPublisher.PublishEvent(ReaderPreferenceChanged{Name: "participantSidebar", Value: TextUtils{}.Choose(service.SidebarOpen, "open", "closed")}, false)
	service.refreshLabels()
	service.load()
}

func (service *ParticipantService) Close() {
	service.close(service.SidebarOpen)
}

func (service *ParticipantService) ToggleThreads() {
	service.ThreadsExpanded = !service.ThreadsExpanded
	service.refreshLabels()
}

func (service *ParticipantService) OpenThread(thread ParticipantThread) {
	service.applicationEventPublisher.PublishEvent(EmailOpenRequested{EmailId: thread.Id}, false)
}

func (ParticipantService) disabled() string { return "disabled" }

func (service *ParticipantService) reset(address string) {
	service.Address = address
	service.Contact = ParticipantContact{}
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
	if service.Address != "" && service.SidebarOpen {
		service.fetch(service.authorization())
	}
}

func (service *ParticipantService) authorization() string {
	return "Bearer " + service.token
}

func (service *ParticipantService) fetch(authorization string) {
	service.Loading = true
	service.Error = ""
	contacts, contactsFailure := service.contactsClient.Search(service.Address, authorization)
	related, relatedFailure := service.searchClient.Messages("from:"+service.Address, authorization)
	github, githubFailure := service.githubClient.Profile(service.Address, authorization)
	service.Contact, service.HasContact = service.matchingContact(contacts)
	service.showGithub(github, githubFailure == nil)
	service.Threads = service.threads(related)
	service.offline(slices.ContainsFunc([]error{contactsFailure, relatedFailure, githubFailure}, func(failure error) bool { return FailureUtils{}.Code(failure) == FailureUtils{}.OfflineCode() }))
	service.Loading = false
	service.refreshLabels()
}

func (service *ParticipantService) matchingContact(contacts []ParticipantContact) (ParticipantContact, bool) {
	index := slices.IndexFunc(contacts, func(contact ParticipantContact) bool { return service.hasAddress(contact.Emails) })
	return service.contactAt(contacts, index)
}

func (service *ParticipantService) hasAddress(emails []ContactValue) bool {
	return slices.ContainsFunc(emails, func(entry ContactValue) bool { return strings.ToLower(entry.Value) == service.Address })
}

func (service *ParticipantService) contactAt(contacts []ParticipantContact, index int) (ParticipantContact, bool) {
	if index < 0 {
		return ParticipantContact{}, false
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
	if loaded && known {
		return status
	}
	return service.unavailable()
}

func (service *ParticipantService) profile(profile *GithubProfile, found bool) (GithubProfile, bool) {
	if profile == nil || !found {
		return GithubProfile{Repositories: []GithubRepository{}}, false
	}
	return *profile, true
}

func (ParticipantService) maximumThreads() int { return 25 }

func (service *ParticipantService) threads(related []RelatedMessage) []ParticipantThread {
	fromSender := slices.DeleteFunc(slices.Clone(related), func(message RelatedMessage) bool { return strings.ToLower(message.Sender) != service.Address })
	sort.SliceStable(fromSender, func(left int, right int) bool { return fromSender[left].ReceivedAt > fromSender[right].ReceivedAt })
	unique := service.unique(fromSender)
	return service.summaries(unique[:min(len(unique), service.maximumThreads())])
}

func (service *ParticipantService) unique(messages []RelatedMessage) []RelatedMessage {
	seen := map[string]bool{}
	kept := []RelatedMessage{}
	for _, message := range messages {
		kept = append(kept, service.firstOf(message, seen)...)
	}
	return kept
}

func (service *ParticipantService) firstOf(message RelatedMessage, seen map[string]bool) []RelatedMessage {
	key := TextUtils{}.Or(TextUtils{}.Or(TextUtils{}.Or(message.ThreadId, message.MessageId), message.Subject), message.Id)
	if key == "" || seen[key] {
		return []RelatedMessage{}
	}
	seen[key] = true
	return []RelatedMessage{message}
}

func (ParticipantService) space() string { return " " }

func (ParticipantService) maximumSnippet() int { return 120 }

func (service *ParticipantService) summaries(messages []RelatedMessage) []ParticipantThread {
	whitespace := regexp.MustCompile(`\s+`)
	threads := []ParticipantThread{}
	for _, message := range messages {
		snippet := []rune(strings.TrimSpace(whitespace.ReplaceAllString(TextUtils{}.Or(message.Snippet, message.Body), service.space())))
		threads = append(threads, ParticipantThread{Id: message.Id, Subject: TextUtils{}.Or(message.Subject, "(No subject)"), Snippet: string(snippet[:min(len(snippet), service.maximumSnippet())]), ReceivedAt: message.ReceivedAt})
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
	service.DisplayName = TextUtils{}.Or(TextUtils{}.Or(service.Contact.Name, strings.TrimSpace(service.Contact.GivenName+service.space()+service.Contact.FamilyName)), service.Address)
	service.Initials = AddressUtils{}.Initials(TextUtils{}.Or(service.DisplayName, service.Address))
	service.HasMoreThreads = len(service.Threads) > service.visibleThreadCount()
	service.VisibleThreads = service.visibleThreads(service.ThreadsExpanded)
}

func (service *ParticipantService) visibleThreads(expanded bool) []ParticipantThread {
	if expanded {
		return service.Threads
	}
	return service.Threads[:min(len(service.Threads), service.visibleThreadCount())]
}
