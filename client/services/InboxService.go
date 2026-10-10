import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// InboxService is the mailbox list: the messages and conversations of one
// folder, their filters, selection, row menu and quick actions. The search
// box, the category picker and the archive tools are their own services; the
// browser work (focus, scrolling, gestures, the address query) is done by
// MailboxBrowserService, which talks to this service through events.
//
// @Service
// @NeedsAll
type InboxService struct {
	// @Required
	Folder string
	// @Needs
	inboxClient *InboxClient
	// @Needs
	threadsClient *ThreadsClient
	// @Needs
	searchClient              *SearchClient
	applicationEventPublisher ApplicationEventPublisher

	// @SessionState
	SelectedFolder string
	// @SessionState
	StatusFilter string
	// @SessionState
	SelectedDomain string
	// @SessionState
	SelectedAccount string
	// @SessionState
	SelectedID string
	// @SessionState
	FocusedID string
	// @SessionState
	ThreadedMode bool
	// @SessionState
	ExpandedThreads map[string]bool
	// @SessionState
	NewMailCount int
	// @LocalState
	Density string

	Loaded               bool
	Error                string
	Search               string
	Selection            []string
	ContextMenu          ContextMenuState
	FolderTitle          string
	UnreadCount          int
	Folders              []FolderChip
	MailboxOptions       []MailboxOption
	SelectedMailboxValue string
	Rows                 []MailRow
	Threads              []MailThreadRow
	SelectedCount        int
	AllVisibleSelected   bool
	DensityLabel         string

	authorization  string
	owner          string
	domains        []string
	mailboxNames   map[string]string
	accounts       []ExternalAccount
	use24HourClock bool
	signedIn       bool
	opened         bool
	allEmails      []Email
	threads        []MailThread
	listTop        float64
	offlineRequest int
	offlineWaiting bool
	offlineFound   []Email
	offlineError   string
	lastScope      string
}

// ---- Loading ---------------------------------------------------------------

// @OnStart
func (inbox *InboxService) Start() {
	inbox.derive()
}

// @OnEvent({"replay":true})
func (inbox *InboxService) SessionChanged(session SessionChanged) {
	inbox.authorization = "Bearer " + session.Token
	inbox.owner = strings.ToLower(strings.TrimSpace(session.Email))
	inbox.domains = inbox.normalizedDomains(session.Domains)
	inbox.mailboxNames = session.MailboxNames
	inbox.accounts = session.ExternalAccounts
	inbox.signedIn = session.Token != ""
	inbox.SelectedAccount = inbox.connectedAccount(inbox.SelectedAccount)
	inbox.SelectedDomain = inbox.allowedDomain()
	inbox.loadWhenReady()
	inbox.derive()
	inbox.settle()
}

// MailboxOpened applies the address the list was opened with: the folder of a
// /folder/<name> page wins, then ?folder=, then the inbox; ?account= or
// ?domain= replace the saved mailbox choice.
//
// @OnEvent({"replay":true})
func (inbox *InboxService) MailboxOpened(opened MailboxOpened) {
	inbox.SelectedFolder = cmp.Or(inbox.Folder, opened.Folder, inbox.inboxFolder())
	inbox.SelectedAccount, inbox.SelectedDomain = inbox.mailboxFromAddress(opened)
	inbox.StatusFilter = cmp.Or(inbox.StatusFilter, inbox.everyMessage())
	inbox.Density = cmp.Or(inbox.Density, inbox.comfortable())
	inbox.SelectedAccount = inbox.patternedAccount(inbox.SelectedAccount)
	inbox.SelectedDomain = inbox.allowedDomain()
	inbox.opened = true
	inbox.loadWhenReady()
	inbox.derive()
	inbox.settle()
}

func (InboxService) inboxFolder() string { return "inbox" }

func (InboxService) everyMessage() string { return "all" }

func (InboxService) comfortable() string { return "comfortable" }

// @OnEvent({"replay":true})
func (inbox *InboxService) MailPreferencesChanged(preferences MailPreferencesChanged) {
	inbox.use24HourClock = preferences.Use24HourClock
	inbox.derive()
}

// Load reads the list again with the current filters: conversations in
// threaded mode, the search route while a search is active, the mailbox
// otherwise. When the server cannot answer, the encrypted offline index does.
func (inbox *InboxService) Load() {
	inbox.Loaded = false
	inbox.Error = ""
	inbox.loaders()[inbox.loadMode()]()
	inbox.Selection = inbox.pruned(inbox.visibleIdentifiers())
	inbox.Loaded = !inbox.offlineWaiting
	inbox.derive()
}

// @OnEvent
func (inbox *InboxService) InboxRefreshRequested(_ InboxRefreshRequested) {
	inbox.Load()
	inbox.settle()
}

// @OnEvent
func (inbox *InboxService) SearchSubmitted(submitted SearchSubmitted) {
	inbox.Search = submitted.Query
	inbox.StatusFilter = map[bool]string{true: inbox.everyMessage(), false: inbox.StatusFilter}[submitted.ResetStatus]
	inbox.Load()
	inbox.settle()
}

// @OnEvent
func (inbox *InboxService) OfflineSearchFinished(finished OfflineSearchFinished) {
	inbox.offlineFound = inbox.offlineCollected(finished)
	inbox.finishOffline(finished)
	inbox.derive()
}

func (inbox *InboxService) loadWhenReady() {
	if inbox.signedIn && inbox.opened {
		inbox.Load()
	}
}

func (inbox *InboxService) loadMode() string {
	switch {
	case inbox.ThreadedMode:
		return inbox.threadedLoad()
	case strings.TrimSpace(inbox.Search) != "":
		return inbox.searchLoad()
	default:
		return inbox.listLoad()
	}
}

func (InboxService) threadedLoad() string { return "threads" }

func (InboxService) searchLoad() string { return "search" }

func (InboxService) listLoad() string { return "list" }

func (inbox *InboxService) loaders() map[string]func() {
	return map[string]func(){inbox.threadedLoad(): inbox.loadThreads, inbox.searchLoad(): inbox.loadSearch, inbox.listLoad(): inbox.loadList}
}

func (inbox *InboxService) loadThreads() {
	found, failure := inbox.threadsClient.List(inbox.authorization, inbox.searchQuery(), inbox.SelectedFolder, inbox.scopeDomain(), inbox.scopeAccount())
	inbox.threads = inbox.threadsAfter(found, failure)
	inbox.searchOfflineWhenUnreachable(failure, "", inbox.failureText(failure))
}

func (inbox *InboxService) loadSearch() {
	found, failure := inbox.searchClient.Search(inbox.authorization, inbox.searchQuery(), inbox.searchLimit(), inbox.scopeDomain(), inbox.scopeAccount())
	inbox.allEmails = inbox.emailsAfter(found, failure)
	inbox.cacheWhenLoaded(failure)
	inbox.searchOfflineWhenFailed(failure, inbox.searchQuery(), "Search is unavailable offline until messages have been synchronized.")
}

func (InboxService) searchLimit() int { return 1000 }

func (inbox *InboxService) loadList() {
	found, failure := inbox.inboxClient.List(inbox.authorization, inbox.readFilter(), inbox.starredFilter(), inbox.scopeDomain(), inbox.scopeAccount())
	inbox.allEmails = inbox.emailsAfter(found, failure)
	inbox.cacheWhenLoaded(failure)
	inbox.searchOfflineWhenFailed(failure, "", inbox.failureText(failure))
	inbox.publishFolderCounts()
}

func (inbox *InboxService) threadsAfter(found []MailThread, failure error) []MailThread {
	switch {
	case failure == nil:
		return found
	case BehaviorErrorCode(failure) == inbox.refused():
		return []MailThread{}
	default:
		return inbox.threads
	}
}

func (InboxService) refused() string { return "HTTP_STATUS" }

func (inbox *InboxService) emailsAfter(found []Email, failure error) []Email {
	if failure != nil {
		return inbox.allEmails
	}
	return found
}

func (inbox *InboxService) failureText(failure error) string {
	switch BehaviorErrorCode(failure) {
	case inbox.refused():
		return "Mailbox is unavailable"
	case "HTTP_RESPONSE":
		return "Unexpected inbox response"
	default:
		return "Failed to load emails"
	}
}

func (inbox *InboxService) searchOfflineWhenUnreachable(failure error, query string, message string) {
	if failure != nil && BehaviorErrorCode(failure) != inbox.refused() {
		inbox.requestOffline(query, message)
	}
}

func (inbox *InboxService) searchOfflineWhenFailed(failure error, query string, message string) {
	if failure != nil {
		inbox.requestOffline(query, message)
	}
}

func (inbox *InboxService) requestOffline(query string, message string) {
	inbox.offlineRequest++
	inbox.offlineWaiting = true
	inbox.offlineFound = []Email{}
	inbox.offlineError = message
	inbox.applicationEventPublisher.PublishEvent(OfflineSearchRequested{
		Request: inbox.offlineRequest, Owner: inbox.owner, Query: query,
		Folder: map[bool]string{true: inbox.SelectedFolder, false: ""}[query == ""],
		Domain: inbox.scopeDomain(), Account: inbox.scopeAccount(),
		Read: inbox.readFilter(), Starred: inbox.starredFilter(), Limit: inbox.offlineLimit(),
	}, false)
}

func (InboxService) offlineLimit() int { return 500 }

func (inbox *InboxService) offlineCollected(finished OfflineSearchFinished) []Email {
	if finished.Request != inbox.offlineRequest {
		return inbox.offlineFound
	}
	return append(inbox.offlineFound, finished.Emails...)
}

func (inbox *InboxService) finishOffline(finished OfflineSearchFinished) {
	if finished.Request != inbox.offlineRequest || !finished.Done || !inbox.offlineWaiting {
		return
	}
	inbox.allEmails = inbox.offlineFound
	inbox.Error = map[bool]string{true: inbox.offlineError, false: ""}[len(inbox.offlineFound) == 0]
	inbox.offlineWaiting = false
	inbox.Loaded = true
	inbox.Selection = inbox.pruned(inbox.visibleIdentifiers())
}

func (inbox *InboxService) cacheWhenLoaded(failure error) {
	if failure == nil {
		inbox.cacheOffline(inbox.allEmails)
	}
}

func (inbox *InboxService) cacheOffline(emails []Email) {
	size := inbox.summarySlice()
	for start := 0; start < len(emails); start += size {
		inbox.applicationEventPublisher.PublishEvent(MailboxSummaries{Owner: inbox.owner, Emails: emails[start:min(start+size, len(emails))]}, false)
	}
}

func (InboxService) summarySlice() int { return 25 }

func (inbox *InboxService) searchQuery() string {
	text := strings.TrimSpace(inbox.Search)
	unread := regexp.MustCompile(`(?i)(^|\s)is:(?:un)?read\b`).MatchString(text)
	starred := regexp.MustCompile(`(?i)(^|\s)is:(?:un)?starred\b`).MatchString(text)
	if text == "" {
		return ""
	} else if inbox.StatusFilter == inbox.unreadStatus() && !unread {
		return strings.TrimSpace(text + " is:unread")
	} else if inbox.StatusFilter == inbox.starredField() && !starred {
		return strings.TrimSpace(text + " is:starred")
	}
	return text
}

func (InboxService) starredField() string { return "starred" }

func (InboxService) unreadStatus() string { return "unread" }

func (inbox *InboxService) readFilter() string {
	return map[bool]string{true: "false", false: ""}[inbox.StatusFilter == inbox.unreadStatus()]
}

func (inbox *InboxService) starredFilter() string {
	return map[bool]string{true: "true", false: ""}[inbox.StatusFilter == inbox.starredField()]
}

func (inbox *InboxService) scopeDomain() string {
	if inbox.SelectedAccount != "" || inbox.SelectedDomain == inbox.everyMailbox() {
		return ""
	}
	return inbox.SelectedDomain
}

func (InboxService) everyMailbox() string { return "all" }

func (inbox *InboxService) scopeAccount() string {
	return inbox.SelectedAccount
}

// ---- The mailbox choice ----------------------------------------------------

// ChangeMailboxFilter picks all mailboxes, one domain ("domain:<name>") or one
// connected account ("account:<id>").
func (inbox *InboxService) ChangeMailboxFilter(value string) {
	inbox.SelectedAccount, inbox.SelectedDomain = inbox.mailboxFromChoice(value)
	inbox.SelectedID = ""
	inbox.Selection = []string{}
	inbox.publishFolderCounts()
	inbox.loadWhenSearching()
	inbox.derive()
}

func (inbox *InboxService) mailboxFromAddress(opened MailboxOpened) (string, string) {
	switch {
	case opened.Account != "":
		return opened.Account, inbox.everyMailbox()
	case opened.Domain != "":
		return "", strings.ToLower(opened.Domain)
	default:
		return inbox.SelectedAccount, cmp.Or(inbox.SelectedDomain, inbox.everyMailbox())
	}
}

func (inbox *InboxService) mailboxFromChoice(value string) (string, string) {
	account, isAccount := strings.CutPrefix(value, inbox.accountPrefix())
	domain, isDomain := strings.CutPrefix(value, inbox.domainPrefix())
	normalized := strings.ToLower(strings.TrimSpace(domain))
	known := slices.ContainsFunc(inbox.accounts, func(connected ExternalAccount) bool { return connected.ID == account })
	if isAccount {
		return map[bool]string{true: account, false: ""}[known], inbox.everyMailbox()
	} else if isDomain && slices.Contains(inbox.domains, normalized) {
		return "", normalized
	}
	return "", inbox.everyMailbox()
}

func (InboxService) accountPrefix() string { return "account:" }

func (InboxService) domainPrefix() string { return "domain:" }

func (inbox *InboxService) loadWhenSearching() {
	if strings.TrimSpace(inbox.Search) != "" || inbox.ThreadedMode {
		inbox.Load()
	}
}

func (inbox *InboxService) patternedAccount(account string) string {
	if !regexp.MustCompile(`^ext_[A-Za-z0-9_-]{20,80}$`).MatchString(account) {
		return ""
	}
	return account
}

func (inbox *InboxService) connectedAccount(account string) string {
	if len(inbox.accounts) > 0 && !slices.ContainsFunc(inbox.accounts, func(known ExternalAccount) bool { return known.ID == account }) {
		return ""
	}
	return account
}

func (inbox *InboxService) allowedDomain() string {
	if inbox.SelectedAccount == "" && (inbox.SelectedDomain == inbox.everyMailbox() || slices.Contains(inbox.domains, inbox.SelectedDomain)) {
		return inbox.SelectedDomain
	}
	return inbox.everyMailbox()
}

func (inbox *InboxService) normalizedDomains(domains []string) []string {
	normalized := []string{}
	for _, domain := range domains {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(domain)))
	}
	return slices.Compact(slices.Sorted(slices.Values(slices.DeleteFunc(normalized, func(domain string) bool { return domain == "" }))))
}

func (inbox *InboxService) mailboxLabel(domain string) string {
	normalized := strings.ToLower(domain)
	return cmp.Or(inbox.mailboxNames[normalized], normalized)
}

// ---- Filters, folders and density ------------------------------------------

// SetStatusFilter shows all, unread or starred messages.
func (inbox *InboxService) SetStatusFilter(status string) {
	inbox.StatusFilter = status
	inbox.Selection = []string{}
	inbox.Load()
}

// ToggleThreadMode switches between messages and conversations.
func (inbox *InboxService) ToggleThreadMode() {
	inbox.ThreadedMode = !inbox.ThreadedMode
	inbox.ExpandedThreads = map[string]bool{}
	inbox.SelectedID = ""
	inbox.Selection = []string{}
	inbox.Load()
}

// SetFolder shows another folder of the loaded mailbox.
func (inbox *InboxService) SetFolder(folder string) {
	inbox.SelectedFolder = cmp.Or(strings.ToLower(strings.TrimSpace(folder)), inbox.inboxFolder())
	inbox.SelectedID = ""
	inbox.Selection = []string{}
	inbox.derive()
}

// ToggleDensity cycles comfortable, compact and default rows.
func (inbox *InboxService) ToggleDensity() {
	next := map[string]string{"comfortable": "compact", "compact": "default", "default": "comfortable"}[inbox.Density]
	inbox.Density = cmp.Or(next, inbox.comfortable())
	inbox.derive()
}

// ---- New mail and scrolling ------------------------------------------------

// @OnEvent
func (inbox *InboxService) NewMailArrived(arrived NewMailArrived) {
	email := arrived.Email
	known := slices.ContainsFunc(inbox.allEmails, inbox.sameID(email.ID))
	valid := inbox.acceptsArrival(email)
	replaced := inbox.replacedEmail(email)
	inbox.allEmails = inbox.withArrival(email, valid, known, replaced)
	inbox.publishFolderCounts()
	visible := slices.ContainsFunc(inbox.visibleEmails(inbox.domainEmails()), inbox.sameID(email.ID))
	inbox.countArrival(valid, known, visible)
	inbox.derive()
}

// ListScrolled clears the new-mail banner at the top and closes the row menu.
func (inbox *InboxService) ListScrolled(top float64) {
	inbox.listTop = top
	inbox.NewMailCount = map[bool]int{true: 0, false: inbox.NewMailCount}[top <= inbox.topSlack()]
	inbox.ContextMenu = ContextMenuState{Targets: []string{}}
	inbox.derive()
}

func (InboxService) topSlack() float64 { return 8 }

// ShowNewMail clears the banner and scrolls to the new messages.
func (inbox *InboxService) ShowNewMail() {
	inbox.NewMailCount = 0
	inbox.applicationEventPublisher.PublishEvent(NewMailShown{}, false)
}

func (inbox *InboxService) acceptsArrival(email Email) bool {
	if email.ID != "" && !inbox.ThreadedMode {
		return true
	}
	return false
}

func (inbox *InboxService) withArrival(email Email, valid bool, known bool, replaced []Email) []Email {
	switch {
	case !valid:
		return inbox.allEmails
	case !known:
		return append([]Email{email}, inbox.allEmails...)
	default:
		return replaced
	}
}

func (inbox *InboxService) countArrival(valid bool, known bool, visible bool) {
	if valid && !known && visible && inbox.listTop > inbox.topSlack() {
		inbox.NewMailCount++
	}
}

// ---- Rows, selection and threads -------------------------------------------

// @OnEvent
func (inbox *InboxService) MailRowActivated(activated MailRowActivated) {
	inbox.selectWhenEmail(activated.EmailID)
	inbox.toggleWhenThread(activated.EmailID, activated.ThreadID)
	inbox.derive()
}

// @OnEvent
func (inbox *InboxService) MailRowFocused(focused MailRowFocused) {
	inbox.FocusedID = focused.ID
	inbox.derive()
}

// ToggleBulkSelection adds a message to the selection, or takes it out.
func (inbox *InboxService) ToggleBulkSelection(id string) {
	inbox.Selection = inbox.toggled(id)
	inbox.derive()
}

// ToggleThreadSelection selects every message of a conversation, or clears
// them when all are selected.
func (inbox *InboxService) ToggleThreadSelection(id string) {
	ids := inbox.threadIdentifiers(id)
	inbox.Selection = inbox.threadToggled(ids, inbox.allSelected(ids))
	inbox.derive()
}

// ToggleAllVisibleSelection selects every visible message, or clears them.
func (inbox *InboxService) ToggleAllVisibleSelection() {
	inbox.Selection = map[bool][]string{true: {}, false: inbox.visibleIdentifiers()}[inbox.allSelected(inbox.visibleIdentifiers())]
	inbox.derive()
}

func (inbox *InboxService) selectWhenEmail(id string) {
	if id != "" {
		inbox.selectEmail(id)
	}
}

func (inbox *InboxService) toggleWhenThread(emailID string, threadID string) {
	if emailID == "" && threadID != "" {
		expanded := map[string]bool{}
		maps.Copy(expanded, inbox.ExpandedThreads)
		expanded[threadID] = !expanded[threadID]
		maps.DeleteFunc(expanded, func(_ string, open bool) bool { return !open })
		inbox.ExpandedThreads = expanded
	}
}

func (inbox *InboxService) selectEmail(id string) {
	inbox.FocusedID = id
	inbox.SelectedID = id
	folder := cmp.Or(inbox.emailByID(inbox.allEmails, id).Folder, inbox.SelectedFolder)
	inbox.publishSelection(id, folder)
}

func (inbox *InboxService) publishSelection(id string, folder string) {
	if folder == "drafts" {
		inbox.applicationEventPublisher.PublishEvent(ComposeRequested{DraftID: id}, false)
		return
	}
	inbox.applicationEventPublisher.PublishEvent(MailSelected{EmailID: id}, false)
}

func (inbox *InboxService) toggled(id string) []string {
	if slices.Contains(inbox.Selection, id) {
		return slices.DeleteFunc(slices.Clone(inbox.Selection), func(selected string) bool { return selected == id })
	}
	return append(slices.Clone(inbox.Selection), id)
}

func (inbox *InboxService) threadToggled(ids []string, every bool) []string {
	if every {
		return slices.DeleteFunc(slices.Clone(inbox.Selection), func(selected string) bool { return slices.Contains(ids, selected) })
	}
	return slices.Compact(slices.Sorted(slices.Values(append(slices.Clone(inbox.Selection), ids...))))
}

func (inbox *InboxService) allSelected(ids []string) bool {
	if len(ids) > 0 && !slices.ContainsFunc(ids, func(id string) bool { return !slices.Contains(inbox.Selection, id) }) {
		return true
	}
	return false
}

func (inbox *InboxService) threadIdentifiers(id string) []string {
	ids := []string{}
	for _, thread := range inbox.visibleThreads() {
		ids = append(ids, inbox.idsWhen(thread.ID == id, thread.Emails)...)
	}
	return ids
}

func (inbox *InboxService) idsWhen(wanted bool, emails []Email) []string {
	if !wanted {
		return []string{}
	}
	return inbox.identifiers(emails)
}

func (inbox *InboxService) pruned(visible []string) []string {
	return slices.DeleteFunc(slices.Clone(inbox.Selection), func(id string) bool { return !slices.Contains(visible, id) })
}

// ---- Mail actions ----------------------------------------------------------

// BulkArchive moves the selected messages to the archive.
func (inbox *InboxService) BulkArchive() {
	inbox.bulk(inbox.toFolder(inbox.archiveFolder()), "Archived selected messages.")
}

func (InboxService) archiveFolder() string { return "archive" }

// BulkTrash moves the selected messages to the trash.
func (inbox *InboxService) BulkTrash() {
	inbox.bulk(inbox.toFolder(inbox.trash()), "Moved selected messages to trash.")
}

func (InboxService) trash() string { return "trash" }

// BulkMarkRead marks the selected messages read.
func (inbox *InboxService) BulkMarkRead() {
	inbox.bulk(map[string]any{inbox.readField(): true}, "Marked selected messages as read.")
}

func (InboxService) readField() string { return "read" }

// BulkMarkUnread marks the selected messages unread.
func (inbox *InboxService) BulkMarkUnread() {
	inbox.bulk(map[string]any{inbox.readField(): false}, "Marked selected messages as unread.")
}

// BulkStar stars the selected messages.
func (inbox *InboxService) BulkStar() {
	inbox.bulk(map[string]any{inbox.starredField(): true}, "Starred selected messages.")
}

// RemoveStars unstars the selected messages.
func (inbox *InboxService) RemoveStars() {
	inbox.bulk(map[string]any{inbox.starredField(): false}, "Unstarred selected messages.")
}

// QuickStar stars or unstars one row.
func (inbox *InboxService) QuickStar(id string) {
	starred, found := inbox.starredState(id)
	inbox.applyWhen(found, []string{id}, map[string]any{inbox.starredField(): !starred}, inbox.starMessage(starred))
}

// QuickArchive archives one row.
func (inbox *InboxService) QuickArchive(id string) {
	inbox.applyWhen(id != "", []string{id}, inbox.toFolder(inbox.archiveFolder()), inbox.archived())
}

func (InboxService) archived() string { return "Archived." }

// QuickTrash moves one row to the trash.
func (inbox *InboxService) QuickTrash(id string) {
	inbox.applyWhen(id != "", []string{id}, inbox.toFolder(inbox.trash()), inbox.movedToTrash())
}

func (InboxService) movedToTrash() string { return "Moved to trash." }

// ReportSpam teaches the spam filter that one message is spam.
func (inbox *InboxService) ReportSpam(id string) {
	updated, failure := inbox.inboxClient.MarkSpam(inbox.authorization, id, map[string]any{inbox.spam(): true})
	replaced, removed := inbox.replacedEmail(updated), inbox.withoutEmail(updated.ID)
	inbox.allEmails = inbox.afterVerdict(failure, inbox.SelectedFolder == inbox.spam(), replaced, removed)
	inbox.toast(map[bool]string{true: "Report spam failed.", false: "Reported spam."}[failure != nil])
	inbox.reloadWhenJudged(failure)
	inbox.derive()
}

func (InboxService) spam() string { return "spam" }

// MarkNotSpam moves a message the filter caught back to the inbox.
func (inbox *InboxService) MarkNotSpam(id string) {
	updated, failure := inbox.inboxClient.MarkSpam(inbox.authorization, id, map[string]any{inbox.spam(): false})
	replaced, removed := inbox.replacedEmail(updated), inbox.withoutEmail(updated.ID)
	inbox.allEmails = inbox.afterVerdict(failure, inbox.SelectedFolder != inbox.spam(), replaced, removed)
	inbox.toast(map[bool]string{true: "Not spam failed.", false: "Moved to inbox."}[failure != nil])
	inbox.reloadWhenJudged(failure)
	inbox.derive()
}

// MailActionApplied merges the changed messages into the list, whoever
// applied the action (this list, the picker, the reader or an undo).
//
// @OnEvent
func (inbox *InboxService) MailActionApplied(applied MailActionApplied) {
	changes := inbox.changesByID(applied.Emails)
	inbox.allEmails = inbox.merged(inbox.allEmails, changes)
	inbox.threads = inbox.mergedThreads(changes)
	inbox.publishFolderCounts()
	inbox.derive()
}

// @OnEvent
func (inbox *InboxService) MailGesture(gesture MailGesture) {
	folder := map[bool]string{true: inbox.trash(), false: inbox.archiveFolder()}[gesture.Folder == inbox.trash()]
	message := map[bool]string{true: inbox.movedToTrash(), false: inbox.archived()}[folder == inbox.trash()]
	ids := inbox.gestureTargets(gesture)
	inbox.applyWhen(len(ids) > 0, ids, inbox.toFolder(folder), message)
	inbox.settle()
}

func (inbox *InboxService) bulk(patch map[string]any, message string) {
	ids := slices.Clone(inbox.Selection)
	applied := inbox.applyWhen(len(ids) > 0, ids, patch, message)
	inbox.Selection = inbox.afterBulk(applied)
	inbox.derive()
}

func (inbox *InboxService) afterBulk(applied bool) []string {
	if applied {
		return []string{}
	}
	return inbox.Selection
}

func (inbox *InboxService) applyWhen(wanted bool, ids []string, patch map[string]any, message string) bool {
	result, failure := inbox.actWhen(wanted, ids, patch)
	return inbox.announce(wanted, result, failure, message)
}

func (inbox *InboxService) actWhen(wanted bool, ids []string, patch map[string]any) (MailActionResult, error) {
	if !wanted {
		return MailActionResult{}, nil
	}
	return inbox.inboxClient.Act(inbox.authorization, map[string]any{"ids": ids, "patch": patch})
}

func (inbox *InboxService) announce(wanted bool, result MailActionResult, failure error, message string) bool {
	if !wanted {
		return false
	} else if failure != nil || result.ActionID == "" {
		inbox.toast("Mail action failed.")
		return false
	}
	inbox.applicationEventPublisher.PublishEvent(MailActionApplied{ActionID: result.ActionID, Message: message, ExpiresAt: result.ExpiresAt, Emails: result.Emails}, false)
	return true
}

func (inbox *InboxService) gestureTargets(gesture MailGesture) []string {
	ids := []string{cmp.Or(inbox.SelectedID, inbox.FocusedID, gesture.EmailID)}
	switch {
	case gesture.Kind == "swipe":
		return []string{gesture.EmailID}
	case len(inbox.Selection) > 0:
		return slices.Clone(inbox.Selection)
	default:
		return slices.DeleteFunc(ids, func(id string) bool { return id == "" })
	}
}

func (inbox *InboxService) starredState(id string) (bool, bool) {
	found := slices.IndexFunc(inbox.allEmails, inbox.sameID(id))
	if found < 0 {
		return false, false
	}
	return inbox.allEmails[found].Starred, true
}

func (inbox *InboxService) afterVerdict(failure error, keep bool, replaced []Email, removed []Email) []Email {
	switch {
	case failure != nil:
		return inbox.allEmails
	case keep:
		return replaced
	default:
		return removed
	}
}

func (inbox *InboxService) reloadWhenJudged(failure error) {
	if failure == nil {
		inbox.Load()
	}
}

func (inbox *InboxService) replacedEmail(updated Email) []Email {
	emails := slices.Clone(inbox.allEmails)
	found := slices.IndexFunc(emails, inbox.sameID(updated.ID))
	if found >= 0 {
		emails[found] = updated
	}
	return emails
}

func (inbox *InboxService) withoutEmail(id string) []Email {
	return slices.DeleteFunc(slices.Clone(inbox.allEmails), inbox.sameID(id))
}

func (inbox *InboxService) changesByID(changes []EmailChange) map[string]EmailChange {
	byID := map[string]EmailChange{}
	for _, change := range changes {
		byID[change.ID] = change
	}
	return byID
}

func (inbox *InboxService) merged(emails []Email, changes map[string]EmailChange) []Email {
	merged := []Email{}
	for _, email := range emails {
		merged = append(merged, inbox.changed(email, changes[email.ID]))
	}
	return merged
}

func (inbox *InboxService) mergedThreads(changes map[string]EmailChange) []MailThread {
	threads := []MailThread{}
	for _, thread := range inbox.threads {
		thread.Emails = inbox.merged(thread.Emails, changes)
		threads = append(threads, thread)
	}
	return threads
}

func (inbox *InboxService) changed(email Email, change EmailChange) Email {
	email.Domain = inbox.text(change.Domain, email.Domain)
	email.Folder = inbox.text(change.Folder, email.Folder)
	email.SystemFolder = inbox.text(change.SystemFolder, email.SystemFolder)
	email.Labels = inbox.labels(change.Labels, email.Labels)
	email.Read = inbox.flag(change.Read, email.Read)
	email.Starred = inbox.flag(change.Starred, email.Starred)
	return email
}

func (InboxService) text(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

func (InboxService) flag(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func (InboxService) labels(value *[]string, fallback []string) []string {
	if value == nil {
		return fallback
	}
	return *value
}

// ---- The row menu ----------------------------------------------------------

// @OnEvent
func (inbox *InboxService) ContextMenuRequested(requested ContextMenuRequested) {
	ids := inbox.menuTargets(requested)
	inbox.ContextMenu = inbox.menuFor(ids, requested.X, requested.Y)
	inbox.announceMenu(ids)
	inbox.derive()
}

// @OnEvent
func (inbox *InboxService) ContextMenuDismissed(_ ContextMenuDismissed) {
	inbox.CloseContextMenu()
}

// CloseContextMenu closes the row menu.
func (inbox *InboxService) CloseContextMenu() {
	inbox.ContextMenu = ContextMenuState{Targets: []string{}}
	inbox.derive()
}

// ContextOpen opens the message the menu belongs to.
func (inbox *InboxService) ContextOpen() {
	ids := inbox.closeMenu()
	inbox.selectWhenEmail(append(ids, "")[0])
	inbox.derive()
}

// ContextArchive archives the menu's messages.
func (inbox *InboxService) ContextArchive() {
	ids := inbox.closeMenu()
	inbox.applyWhen(len(ids) > 0, ids, inbox.toFolder(inbox.archiveFolder()), inbox.archived())
	inbox.derive()
}

// ContextTrash moves the menu's messages to the trash.
func (inbox *InboxService) ContextTrash() {
	ids := inbox.closeMenu()
	inbox.applyWhen(len(ids) > 0, ids, inbox.toFolder(inbox.trash()), inbox.movedToTrash())
	inbox.derive()
}

// ContextToggleRead marks the menu's messages read, or unread when the first
// one is read.
func (inbox *InboxService) ContextToggleRead() {
	read, _ := inbox.menuEmailState()
	ids := inbox.closeMenu()
	inbox.applyWhen(len(ids) > 0, ids, map[string]any{inbox.readField(): !read}, inbox.readMessage(read))
	inbox.derive()
}

// ContextToggleStar stars the menu's messages, or unstars them when the first
// one is starred.
func (inbox *InboxService) ContextToggleStar() {
	_, starred := inbox.menuEmailState()
	ids := inbox.closeMenu()
	inbox.applyWhen(len(ids) > 0, ids, map[string]any{inbox.starredField(): !starred}, inbox.starMessage(starred))
	inbox.derive()
}

// ContextMove opens the folder picker for the menu's messages.
func (inbox *InboxService) ContextMove() {
	inbox.openPicker(inbox.move(), inbox.closeMenu())
}

func (InboxService) move() string { return "move" }

// ContextLabels opens the label picker for the menu's messages.
func (inbox *InboxService) ContextLabels() {
	inbox.openPicker(inbox.label(), inbox.closeMenu())
}

func (InboxService) label() string { return "label" }

func (inbox *InboxService) menuTargets(requested ContextMenuRequested) []string {
	if requested.EmailID != "" {
		return []string{requested.EmailID}
	}
	return inbox.threadIdentifiers(requested.ThreadID)
}

func (inbox *InboxService) menuFor(ids []string, x float64, y float64) ContextMenuState {
	if len(ids) == 0 {
		return inbox.ContextMenu
	}
	return ContextMenuState{Open: true, Targets: slices.Compact(slices.Clone(ids)), X: x, Y: y}
}

func (inbox *InboxService) announceMenu(ids []string) {
	if len(ids) > 0 {
		inbox.applicationEventPublisher.PublishEvent(ContextMenuOpened{Count: len(ids)}, false)
	}
}

func (inbox *InboxService) closeMenu() []string {
	ids := slices.Clone(inbox.ContextMenu.Targets)
	inbox.ContextMenu = ContextMenuState{Targets: []string{}}
	return ids
}

func (inbox *InboxService) menuEmailState() (bool, bool) {
	email := inbox.emailByID(inbox.everyEmail(), append(slices.Clone(inbox.ContextMenu.Targets), "")[0])
	return email.Read, email.Starred
}

func (inbox *InboxService) everyEmail() []Email {
	every := slices.Clone(inbox.allEmails)
	for _, thread := range inbox.threads {
		every = append(every, thread.Emails...)
	}
	return every
}

func (inbox *InboxService) emailByID(emails []Email, id string) Email {
	found := slices.IndexFunc(emails, inbox.sameID(id))
	if found < 0 {
		return Email{}
	}
	return emails[found]
}

func (inbox *InboxService) menuLabels(menu ContextMenuState) ContextMenuState {
	read, starred := inbox.menuEmailState()
	count := len(menu.Targets)
	menu.Heading = map[bool]string{true: "Message actions", false: fmt.Sprintf("%d message actions", count)}[count == 1]
	menu.ReadLabel = map[bool]string{true: "Mark unread", false: "Mark read"}[read]
	menu.StarLabel = map[bool]string{true: "Unstar", false: "Star"}[starred]
	return menu
}

// ---- The category picker ---------------------------------------------------

// OpenBulkMovePicker opens the folder picker for the selected messages.
func (inbox *InboxService) OpenBulkMovePicker() {
	inbox.openPicker(inbox.move(), inbox.Selection)
}

// OpenBulkLabelPicker opens the label picker for the selected messages.
func (inbox *InboxService) OpenBulkLabelPicker() {
	inbox.openPicker(inbox.label(), inbox.Selection)
}

// @OnEvent
func (inbox *InboxService) CategoryPickerApplied(_ CategoryPickerApplied) {
	inbox.Selection = []string{}
	inbox.derive()
}

func (inbox *InboxService) openPicker(mode string, ids []string) {
	inbox.applicationEventPublisher.PublishEvent(CategoryPickerRequested{Mode: mode, Messages: inbox.pickerMessages(ids)}, false)
	inbox.derive()
}

func (inbox *InboxService) pickerMessages(ids []string) []PickerMessage {
	byID := map[string]PickerMessage{}
	for _, email := range slices.DeleteFunc(inbox.everyEmail(), func(email Email) bool { return !slices.Contains(ids, email.ID) }) {
		byID[email.ID] = PickerMessage{ID: email.ID, Domain: email.Domain, Folder: email.Folder, Labels: email.Labels}
	}
	return slices.SortedFunc(maps.Values(byID), func(left PickerMessage, right PickerMessage) int {
		return slices.Index(ids, left.ID) - slices.Index(ids, right.ID)
	})
}

// ---- Keyboard shortcuts ----------------------------------------------------

// @OnEvent
func (inbox *InboxService) MailShortcut(shortcut MailShortcut) {
	inbox.runShortcut(shortcut.Name)
	inbox.derive()
	inbox.settle()
}

func (inbox *InboxService) runShortcut(name string) {
	if action, found := inbox.shortcuts()[name]; found {
		action()
	}
}

func (inbox *InboxService) shortcuts() map[string]func() {
	return map[string]func(){
		"navigation:next-email":  func() { inbox.moveSelection(1) },
		"navigation:prev-email":  func() { inbox.moveSelection(-1) },
		"navigation:scroll-down": func() { inbox.scrollList(inbox.scrollStep()) },
		"navigation:scroll-up":   func() { inbox.scrollList(-inbox.scrollStep()) },
		"email:star":             func() { inbox.withSelected(inbox.QuickStar) },
		"email:archive":          func() { inbox.withSelected(inbox.archiveSelected) },
		"email:trash":            func() { inbox.withSelected(inbox.trashSelected) },
		"email:report-spam":      func() { inbox.withSelected(inbox.reportSelected) },
		"email:mark-read":        func() { inbox.withSelected(inbox.readSelected) },
		"email:mark-unread":      func() { inbox.withSelected(inbox.unreadSelected) },
		"email:open":             func() { inbox.openSelected() },
		"core:select-all":        func() { inbox.Selection = inbox.visibleIdentifiers() },
		"core:deselect-all":      func() { inbox.SelectedID, inbox.Selection = "", []string{} },
	}
}

func (InboxService) scrollStep() int { return 200 }

func (inbox *InboxService) withSelected(action func(string)) {
	if inbox.SelectedID != "" && slices.ContainsFunc(inbox.allEmails, inbox.sameID(inbox.SelectedID)) {
		action(inbox.SelectedID)
	}
}

func (inbox *InboxService) archiveSelected(id string) {
	inbox.QuickArchive(id)
	inbox.SelectedID = ""
}

func (inbox *InboxService) trashSelected(id string) {
	inbox.QuickTrash(id)
	inbox.SelectedID = ""
}

func (inbox *InboxService) reportSelected(id string) {
	inbox.ReportSpam(id)
	inbox.SelectedID = ""
}

func (inbox *InboxService) readSelected(id string) {
	inbox.applyWhen(true, []string{id}, map[string]any{inbox.readField(): true}, inbox.readMessage(false))
}

func (inbox *InboxService) unreadSelected(id string) {
	inbox.applyWhen(true, []string{id}, map[string]any{inbox.readField(): false}, inbox.readMessage(true))
}

func (inbox *InboxService) openSelected() {
	if inbox.SelectedID != "" {
		inbox.applicationEventPublisher.PublishEvent(EmailOpenRequested{EmailID: inbox.SelectedID}, false)
	}
}

func (inbox *InboxService) moveSelection(direction int) {
	inbox.selectWithin(inbox.visibleEmails(inbox.domainEmails()), direction)
}

func (inbox *InboxService) selectWithin(emails []Email, direction int) {
	if len(emails) == 0 {
		return
	}
	current := slices.IndexFunc(emails, inbox.sameID(inbox.SelectedID))
	inbox.selectEmail(emails[max(0, min(current+direction, len(emails)-1))].ID)
	inbox.applicationEventPublisher.PublishEvent(SelectionMoved{EmailID: inbox.SelectedID}, false)
}

func (inbox *InboxService) scrollList(amount int) {
	inbox.applicationEventPublisher.PublishEvent(ListScrollRequested{Amount: amount}, false)
}

// ---- What the view shows ---------------------------------------------------

func (inbox *InboxService) derive() {
	domainEmails := inbox.domainEmails()
	emails := inbox.visibleEmails(domainEmails)
	threads := inbox.visibleThreads()
	active := inbox.activeRow(inbox.rovingOrder(emails, threads))
	inbox.FolderTitle = inbox.title(cmp.Or(inbox.SelectedFolder, inbox.inboxFolder()))
	inbox.UnreadCount = len(slices.DeleteFunc(slices.Clone(emails), func(email Email) bool { return email.Read }))
	inbox.Folders = inbox.folderChips(domainEmails)
	inbox.MailboxOptions = slices.Concat(inbox.domainOptions(), inbox.accountOptions())
	inbox.SelectedMailboxValue = inbox.mailboxValue()
	inbox.Rows = inbox.rows(emails, active)
	inbox.Threads = inbox.threadRows(threads, active)
	inbox.SelectedCount = len(inbox.Selection)
	inbox.AllVisibleSelected = inbox.allSelected(inbox.visibleIdentifiers())
	inbox.DensityLabel = inbox.title(inbox.Density)
	inbox.ContextMenu = inbox.menuLabels(inbox.ContextMenu)
	inbox.publishScopeWhenChanged(len(emails))
}

func (inbox *InboxService) logicalFolder(email Email) string {
	return cmp.Or(email.SystemFolder, email.Folder, inbox.inboxFolder())
}

func (inbox *InboxService) domainEmails() []Email {
	return slices.DeleteFunc(slices.Clone(inbox.allEmails), func(email Email) bool { return !inbox.inSelectedMailbox(email) })
}

func (inbox *InboxService) inSelectedMailbox(email Email) bool {
	account := email.ProviderAccountID == inbox.SelectedAccount
	hosted := map[bool]bool{true: strings.ToLower(email.Domain) == inbox.SelectedDomain}[email.ProviderAccountID == ""]
	every := slices.Contains([]string{inbox.everyMailbox(), ""}, inbox.SelectedDomain)
	return map[bool]bool{true: account, false: map[bool]bool{true: true, false: hosted}[every]}[inbox.SelectedAccount != ""]
}

func (inbox *InboxService) visibleEmails(domainEmails []Email) []Email {
	if strings.TrimSpace(inbox.Search) != "" {
		return domainEmails
	}
	return slices.DeleteFunc(slices.Clone(domainEmails), func(email Email) bool { return inbox.logicalFolder(email) != inbox.SelectedFolder })
}

func (inbox *InboxService) visibleThreads() []MailThread {
	threads := []MailThread{}
	for _, thread := range inbox.threads {
		thread.Emails = slices.DeleteFunc(slices.Clone(thread.Emails), func(email Email) bool { return !inbox.inSelectedMailbox(email) })
		threads = append(threads, thread)
	}
	return slices.DeleteFunc(threads, func(thread MailThread) bool { return len(thread.Emails) == 0 })
}

func (inbox *InboxService) visibleIdentifiers() []string {
	return inbox.unique(inbox.orderedIdentifiers())
}

func (inbox *InboxService) orderedIdentifiers() []string {
	flat := inbox.identifiers(inbox.visibleEmails(inbox.domainEmails()))
	threaded := inbox.threadMessageIdentifiers(inbox.visibleThreads())
	return map[bool][]string{true: threaded, false: flat}[inbox.ThreadedMode]
}

func (inbox *InboxService) threadMessageIdentifiers(threads []MailThread) []string {
	ids := []string{}
	for _, thread := range threads {
		ids = append(ids, inbox.identifiers(thread.Emails)...)
	}
	return ids
}

func (inbox *InboxService) rovingOrder(emails []Email, threads []MailThread) []string {
	ids := []string{}
	for _, thread := range threads {
		ids = append(ids, thread.ID)
		ids = append(ids, inbox.idsWhen(inbox.ExpandedThreads[thread.ID], thread.Emails)...)
	}
	return map[bool][]string{true: ids, false: inbox.identifiers(emails)}[inbox.ThreadedMode]
}

func (inbox *InboxService) activeRow(ids []string) string {
	if slices.Contains(ids, inbox.FocusedID) {
		return inbox.FocusedID
	}
	return append(slices.Clone(ids), "")[0]
}

func (inbox *InboxService) rows(emails []Email, active string) []MailRow {
	rows := []MailRow{}
	for _, email := range emails {
		rows = append(rows, inbox.row(email, active))
	}
	return rows
}

func (inbox *InboxService) row(email Email, active string) MailRow {
	return MailRow{
		ID: email.ID, Sender: email.Sender, Subject: email.Subject, Folder: email.Folder, Read: email.Read,
		Starred: email.Starred, Labels: email.Labels, HasAttachments: email.HasAttachments,
		Spam: inbox.isSpam(email), SpamScoreLabel: inbox.spamScoreLabel(email), SnoozeLabel: inbox.snoozeLabel(email),
		TimeLabel: DateLabelUtils{}.Numeric(email.ReceivedAt), Initials: inbox.initials(email.Sender),
		BulkSelected: slices.Contains(inbox.Selection, email.ID),
		TabIndex:     map[bool]int{true: 0, false: -1}[email.ID == active],
	}
}

func (inbox *InboxService) threadRows(threads []MailThread, active string) []MailThreadRow {
	rows := []MailThreadRow{}
	for _, thread := range threads {
		latest := thread.Emails[len(thread.Emails)-1]
		rows = append(rows, MailThreadRow{
			ID: thread.ID, Subject: thread.Subject, Count: len(thread.Emails), Sender: latest.Sender,
			Initials: inbox.initials(latest.Sender), SnoozeLabel: inbox.snoozeLabel(latest),
			TimeLabel: DateLabelUtils{}.Numeric(latest.ReceivedAt), PersonalLevel: thread.PersonalLevel,
			PersonalLevelLabel: map[int]string{inbox.onlyYou(): "Sent only to you", 1: "Sent to you and others"}[thread.PersonalLevel],
			Expanded:           inbox.ExpandedThreads[thread.ID], Selected: inbox.allSelected(inbox.identifiers(thread.Emails)),
			TabIndex: map[bool]int{true: 0, false: -1}[thread.ID == active], Emails: inbox.rows(thread.Emails, active),
		})
	}
	return rows
}

func (InboxService) onlyYou() int { return 2 }

func (inbox *InboxService) isSpam(email Email) bool {
	if inbox.logicalFolder(email) == inbox.spam() || email.SpamScore != nil || email.SpamReasons != nil {
		return true
	}
	return false
}

func (InboxService) spamScoreLabel(email Email) string {
	if email.SpamScore == nil {
		return ""
	}
	return fmt.Sprintf("Score %.1f", *email.SpamScore)
}

func (inbox *InboxService) snoozeLabel(email Email) string {
	if email.Folder != "snoozed" || email.SnoozedUntil == "" {
		return ""
	}
	return SnoozePresetsUtils{}.UntilLabel(email.SnoozedUntil, time.Now(), inbox.use24HourClock)
}

func (inbox *InboxService) initials(address string) string {
	local := cmp.Or(strings.Split(address, "@")[0], "?")
	runes := []rune(local)
	return strings.ToUpper(string(runes[:min(inbox.initialsLength(), utf8.RuneCountInString(local))]))
}

func (InboxService) initialsLength() int { return 2 }

func (inbox *InboxService) folderChips(domainEmails []Email) []FolderChip {
	chips := []FolderChip{}
	for _, folder := range inbox.folderNames(domainEmails) {
		count := len(slices.DeleteFunc(slices.Clone(domainEmails), func(email Email) bool { return inbox.logicalFolder(email) != folder }))
		chips = append(chips, FolderChip{Name: folder, Label: fmt.Sprintf("%s (%d)", folder, count)})
	}
	return chips
}

func (inbox *InboxService) folderNames(domainEmails []Email) []string {
	fixed := []string{inbox.inboxFolder(), inbox.archiveFolder(), "snoozed", inbox.trash()}
	custom := []string{}
	for _, email := range domainEmails {
		custom = append(custom, inbox.logicalFolder(email))
	}
	custom = slices.DeleteFunc(custom, func(folder string) bool { return slices.Contains(fixed, folder) })
	return append(fixed, slices.Compact(slices.Sorted(slices.Values(custom)))...)
}

func (inbox *InboxService) domainOptions() []MailboxOption {
	options := []MailboxOption{}
	for _, domain := range inbox.domains {
		options = append(options, MailboxOption{Value: inbox.domainPrefix() + domain, Label: inbox.mailboxLabel(domain)})
	}
	return options
}

func (inbox *InboxService) accountOptions() []MailboxOption {
	options := []MailboxOption{}
	for _, account := range inbox.accounts {
		options = append(options, MailboxOption{Value: inbox.accountPrefix() + account.ID, Label: cmp.Or(account.Label, account.MailboxAddress)})
	}
	return options
}

func (inbox *InboxService) mailboxValue() string {
	if inbox.SelectedAccount != "" {
		return inbox.accountPrefix() + inbox.SelectedAccount
	} else if inbox.SelectedDomain != inbox.everyMailbox() && inbox.SelectedDomain != "" {
		return inbox.domainPrefix() + inbox.SelectedDomain
	}
	return inbox.everyMailbox()
}

func (inbox *InboxService) accountLabel() string {
	found := slices.IndexFunc(inbox.accounts, func(account ExternalAccount) bool { return account.ID == inbox.SelectedAccount })
	return inbox.labelOfAccount(found)
}

func (inbox *InboxService) labelOfAccount(found int) string {
	if found < 0 {
		return ""
	}
	return cmp.Or(inbox.accounts[found].Label, inbox.accounts[found].MailboxAddress)
}

func (inbox *InboxService) publishScopeWhenChanged(shown int) {
	inbox.publishWhenChanged(inbox.scope(inbox.exportTotal(shown)))
}

func (inbox *InboxService) exportTotal(shown int) int {
	if strings.TrimSpace(inbox.Search) == "" && inbox.StatusFilter == inbox.everyMessage() {
		return shown
	}
	return 0
}

func (inbox *InboxService) scope(total int) MailboxScopeChanged {
	return MailboxScopeChanged{
		Folder: inbox.SelectedFolder, FolderTitle: inbox.FolderTitle, StatusFilter: inbox.StatusFilter,
		Domain: inbox.SelectedDomain, Account: inbox.SelectedAccount, AccountLabel: inbox.accountLabel(),
		DomainLabel: inbox.mailboxLabel(inbox.SelectedDomain), Domains: inbox.domains, Owner: inbox.owner,
		Total: total, Searching: strings.TrimSpace(inbox.Search) != "",
	}
}

func (inbox *InboxService) publishWhenChanged(scope MailboxScopeChanged) {
	encoded, _ := json.Marshal(scope)
	if string(encoded) != inbox.lastScope {
		inbox.lastScope = string(encoded)
		inbox.applicationEventPublisher.PublishEvent(scope, true)
	}
}

func (inbox *InboxService) publishFolderCounts() {
	unfiltered := map[bool]bool{true: inbox.StatusFilter == inbox.everyMessage()}[strings.TrimSpace(inbox.Search) == ""]
	inbox.publishCountsWhen(unfiltered, inbox.folderCounts())
}

func (inbox *InboxService) publishCountsWhen(unfiltered bool, counts FolderCountsChanged) {
	if unfiltered {
		inbox.applicationEventPublisher.PublishEvent(counts, false)
	}
}

func (inbox *InboxService) folderCounts() FolderCountsChanged {
	counts := FolderCountsChanged{Counts: map[string]int{}, ByDomain: map[string]map[string]int{}, ByAccount: map[string]map[string]int{}, UnreadInboxByDomain: map[string]int{}}
	for _, email := range inbox.allEmails {
		folder := inbox.logicalFolder(email)
		domain := cmp.Or(strings.ToLower(email.Domain), "unknown")
		counts.Counts[folder]++
		scoped := map[bool]map[string]map[string]int{true: counts.ByAccount, false: counts.ByDomain}[email.ProviderAccountID != ""]
		key := cmp.Or(email.ProviderAccountID, domain)
		scoped[key] = inbox.counted(scoped[key], folder)
		unread := map[bool]int{true: 1}[folder == inbox.inboxFolder()] * map[bool]int{true: 0, false: 1}[email.Read]
		counts.UnreadInbox += unread
		counts.UnreadInboxByDomain[domain] += unread * map[bool]int{true: 0, false: 1}[email.ProviderAccountID != ""]
	}
	maps.DeleteFunc(counts.UnreadInboxByDomain, func(_ string, count int) bool { return count == 0 })
	return counts
}

func (InboxService) counted(folders map[string]int, folder string) map[string]int {
	next := map[string]int{folder: 0}
	maps.Copy(next, folders)
	next[folder]++
	return next
}

// ---- Small shared pieces ---------------------------------------------------

func (inbox *InboxService) toast(message string) {
	inbox.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (inbox *InboxService) settle() {
	inbox.applicationEventPublisher.PublishEvent(ViewSettled{Source: "inbox"}, false)
}

func (InboxService) sameID(id string) func(Email) bool {
	return func(email Email) bool { return email.ID == id }
}

func (InboxService) identifiers(emails []Email) []string {
	ids := []string{}
	for _, email := range emails {
		ids = append(ids, email.ID)
	}
	return ids
}

func (InboxService) unique(ids []string) []string {
	seen := map[string]bool{"": true}
	return slices.DeleteFunc(ids, func(id string) bool {
		repeated := seen[id]
		seen[id] = true
		return repeated
	})
}

func (InboxService) title(name string) string {
	runes := []rune(name)
	return strings.ToUpper(string(runes[:min(1, len(runes))])) + string(runes[min(1, len(runes)):])
}

func (inbox *InboxService) toFolder(folder string) map[string]any {
	return map[string]any{"folder": folder}
}

func (InboxService) readMessage(read bool) string {
	return map[bool]string{true: "Marked as unread.", false: "Marked as read."}[read]
}

func (InboxService) starMessage(starred bool) string {
	return map[bool]string{true: "Unstarred.", false: "Starred."}[starred]
}
