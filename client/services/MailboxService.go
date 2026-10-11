import (
	"slices"
	"strconv"
	"strings"
	"time"
)

// MailboxService holds the sidebar's mailbox data: the signed-in domains and
// their personal labels, the per-folder badges and the Activity badge. It
// also applies the mailbox actions the shell owns (drops on a folder, spam
// reports, undo and redo), which other services ask for with
// MailActionRequested and SpamReportRequested.
// @Service
type MailboxService struct {
	// @Needs
	inboxClient *InboxClient
	// @Needs
	calendarClient *CalendarClient
	// @Needs
	activityClient *ActivityClient
	applicationEventPublisher ApplicationEventPublisher
	Domains                   []string
	MailboxNames              map[string]string
	Counts                    map[string]int
	CountsByDomain            map[string]map[string]int
	CountsByAccount           map[string]map[string]int
	ShowUnreadForAllFolders   bool
	badgeScope                string
	badgeDomain               string
	pendingAction             NotificationActionReceived
	token                     string
	email                     string
	activitySince             string
}

// @OnStart
func (mailbox *MailboxService) Start() {
	mailbox.Domains = []string{}
	mailbox.MailboxNames = map[string]string{}
	mailbox.Counts = map[string]int{}
	mailbox.CountsByDomain = map[string]map[string]int{}
	mailbox.CountsByAccount = map[string]map[string]int{}
}

// HearSession loads the badges for the signed-in user and keeps the Activity
// badge current every minute.
// @OnEvent({"replay":true})
func (mailbox *MailboxService) HearSession(changed SessionChanged) {
	mailbox.token = changed.Token
	mailbox.email = strings.ToLower(changed.Email)
	mailbox.Domains = append([]string{}, changed.Domains...)
	FlowUtils{}.Either(changed.SignedIn, mailbox.load, func() { mailbox.publishBadge(0) })()
}

// HearStorage restores the cached mailbox labels and when Activity was last
// viewed on this device.
// @OnEvent({"replay":true})
func (mailbox *MailboxService) HearStorage(stored StoredValues) {
	mailbox.MailboxNames = MailboxUtils{}.Names(stored.Session[mailbox.names().LabelsKey], mailbox.Domains)
	mailbox.activitySince = stored.Local[mailbox.activityKey()]
}

// HearPreferences applies the user's mailbox labels and badge preference.
// @OnEvent({"replay":true})
func (mailbox *MailboxService) HearPreferences(loaded PreferencesLoaded) {
	mailbox.MailboxNames = MailboxUtils{}.Allowed(loaded.MailboxNames, mailbox.Domains)
	mailbox.ShowUnreadForAllFolders = loaded.ShowUnreadForAllFolders
	mailbox.badgeScope = loaded.UnreadBadgeScope
	mailbox.badgeDomain = loaded.UnreadBadgeDomain
	mailbox.applicationEventPublisher.PublishEvent(StorageWritten{Area: "session", Values: map[string]string{mailbox.names().LabelsKey: MailboxUtils{}.Encoded(mailbox.MailboxNames)}, Removed: []string{}}, false)
}

// HearCounts takes the badges that the inbox list counted.
// @OnEvent
func (mailbox *MailboxService) HearCounts(counted FolderCountsChanged) {
	mailbox.Counts = MailboxUtils{}.WithActivity(counted.Counts, mailbox.Counts[mailbox.names().Activity])
	mailbox.CountsByDomain = counted.ByDomain
	mailbox.CountsByAccount = counted.ByAccount
	mailbox.publishBadge(MailboxUtils{}.BadgeCount(counted, BadgePreference{Scope: mailbox.badgeScope, Domain: mailbox.badgeDomain}, mailbox.Domains))
}

// HearNotificationAction applies the Mark read or Archive button of a new-mail
// notification.
// @OnEvent
func (mailbox *MailboxService) HearNotificationAction(received NotificationActionReceived) {
	mailbox.pendingAction = received
	FlowUtils{}.When(mailbox.token != "", mailbox.applyNotificationAction)()
}

// HearRefresh counts again after mail arrived or a mailbox action.
// @OnEvent
func (mailbox *MailboxService) HearRefresh(request InboxRefreshRequested) {
	FlowUtils{}.When(mailbox.token != "", mailbox.refreshCounts)()
}

// HearTimer refreshes the Activity badge every minute.
// @OnEvent
func (mailbox *MailboxService) HearTimer(elapsed TimerElapsed) {
	FlowUtils{}.When(mailbox.activityDue(elapsed.Name), mailbox.refreshActivity)()
}

// HearActivityViewed clears the Activity badge once the page has been seen.
// @OnEvent
func (mailbox *MailboxService) HearActivityViewed(viewed ActivityViewed) {
	mailbox.Counts = MailboxUtils{}.WithActivity(mailbox.Counts, 0)
}

// HearDrop moves the dragged messages into the folder they were dropped on.
// @OnEvent
func (mailbox *MailboxService) HearDrop(dropped FolderDropped) {
	mailbox.apply(dropped.Ids, map[string]any{"folder": dropped.Folder}, MailboxUtils{}.MovedMessage(len(dropped.Ids), dropped.Folder, dropped.Domain))
}

// HearMailAction applies a mailbox action that another service asked for.
// @OnEvent
func (mailbox *MailboxService) HearMailAction(request MailActionRequested) {
	mailbox.apply(request.Ids, request.Patch, request.Message)
}

// HearSpamReport reports a message as spam, or as not spam.
// @OnEvent
func (mailbox *MailboxService) HearSpamReport(request SpamReportRequested) {
	_, failure := mailbox.inboxClient.MarkSpam(mailbox.bearer(), request.ID, SpamMark{Spam: request.Spam})
	FlowUtils{}.Either(failure == nil, func() { mailbox.reported(MailboxUtils{}.SpamDone(request.Spam)) }, func() {
		mailbox.toast(ToastRequested{Message: MailboxUtils{}.SpamFailure(request.Spam)})
	})()
}

// HearToastAction undoes or redoes a mailbox action from its toast.
// @OnEvent
func (mailbox *MailboxService) HearToastAction(chosen ToastActionChosen) {
	mailbox.toastAction(chosen.Action)(chosen.Subject)
}

// FolderCount is the badge of an aggregate folder row, or empty for none.
func (mailbox *MailboxService) FolderCount(name string) string {
	return mailbox.badge(mailbox.shown(name), mailbox.Counts[name])
}

// DomainFolderCount is the badge of one folder of one domain.
func (mailbox *MailboxService) DomainFolderCount(folder string, domain string) string {
	return mailbox.badge(mailbox.shown(folder), mailbox.CountsByDomain[domain][folder])
}

// AccountFolderCount is the badge of one folder of one connected account.
func (mailbox *MailboxService) AccountFolderCount(folder string, account string) string {
	return mailbox.badge(mailbox.shown(folder), mailbox.CountsByAccount[account][folder])
}

// MailboxLabel is the personal label of a domain, or the domain itself.
func (mailbox *MailboxService) MailboxLabel(domain string) string {
	normalized := strings.ToLower(strings.TrimSpace(domain))
	return TextUtils{}.FirstOf(mailbox.MailboxNames[normalized], normalized)
}

// CustomFoldersForDomain lists a domain's folders that are not system folders.
func (mailbox *MailboxService) CustomFoldersForDomain(domain string) []string {
	return FolderUtils{}.Custom(mailbox.CountsByDomain[domain], []string{mailbox.names().Inbox, "drafts", "sent", "scheduled", "reminders", "snoozed", "archive", "spam", "trash", mailbox.names().Activity})
}

// FolderLabel title-cases a folder path for the sidebar.
func (mailbox *MailboxService) FolderLabel(name string) string {
	return FolderUtils{}.Label(name)
}

// applyNotificationAction applies a notification button once the session is
// known, because the launch address can be read before the session.
func (mailbox *MailboxService) applyNotificationAction() {
	received := mailbox.pendingAction
	mailbox.pendingAction = NotificationActionReceived{}
	patch, message := MailboxUtils{}.NotificationAction(received.Action)
	FlowUtils{}.When(!slices.Contains([]bool{MailboxUtils{}.SafeID(received.EmailID), patch != nil}, false), func() { mailbox.apply([]string{received.EmailID}, patch, message) })()
}

func (mailbox *MailboxService) load() {
	mailbox.applyNotificationAction()
	mailbox.refreshCounts()
	mailbox.refreshActivity()
	mailbox.applicationEventPublisher.PublishEvent(TimerRequested{Name: mailbox.names().Activity, DelayMs: 60000, Repeat: true}, false)
}

// refreshCounts tallies /inbox per folder, domain and connected account, and
// counts the calendar events of the next two weeks.
func (mailbox *MailboxService) refreshCounts() {
	summaries, failure := mailbox.inboxClient.Summaries(mailbox.bearer())
	FlowUtils{}.When(HTTPUtils{}.Refused(failure), func() {
		mailbox.applicationEventPublisher.PublishEvent(SessionCheckRequested{Reason: mailbox.names().Inbox}, false)
	})()
	counted := MailboxUtils{}.Tally(summaries)
	counted.Counts["calendar"] = mailbox.upcomingEvents()
	mailbox.applicationEventPublisher.PublishEvent(counted, false)
}

func (mailbox *MailboxService) upcomingEvents() int {
	now := time.Now().UTC()
	events, _ := mailbox.calendarClient.Events(mailbox.bearer(), now.Format(time.RFC3339Nano), now.AddDate(0, 0, 14).Format(time.RFC3339Nano))
	return len(events)
}

// refreshActivity reads how many recipient events arrived since this mailbox
// last viewed Activity. A first visit starts at now, so old events do not
// appear as a wall of new ones.
func (mailbox *MailboxService) refreshActivity() {
	FlowUtils{}.When(mailbox.activitySince == "", mailbox.startActivity)()
	summary, failure := mailbox.activityClient.Unseen(mailbox.bearer(), "all", "1", mailbox.activitySince)
	FlowUtils{}.When(failure == nil, func() { mailbox.Counts = MailboxUtils{}.WithActivity(mailbox.Counts, max(0, summary.UnseenCount)) })()
}

func (mailbox *MailboxService) startActivity() {
	mailbox.activitySince = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	mailbox.applicationEventPublisher.PublishEvent(StorageWritten{Area: "local", Values: map[string]string{mailbox.activityKey(): mailbox.activitySince}, Removed: []string{}}, false)
}

// apply runs one exact-owner mailbox action and offers its undo.
func (mailbox *MailboxService) apply(ids []string, patch map[string]any, message string) {
	requestID := IdentityUtils{}.UUID(strings.Join(ids, ","))
	result, failure := mailbox.inboxClient.Apply(mailbox.bearer(), MailActionRequest{Ids: ids, Patch: patch, ClientRequestID: requestID})
	mailbox.settled(failure, result, message, mailbox.names().Undo)()
}

func (mailbox *MailboxService) undo(actionID string) {
	result, failure := mailbox.inboxClient.Undo(mailbox.bearer(), actionID)
	mailbox.settled(failure, result, "Action undone.", mailbox.names().Redo)()
}

func (mailbox *MailboxService) redo(actionID string) {
	result, failure := mailbox.inboxClient.Redo(mailbox.bearer(), actionID)
	mailbox.settled(failure, result, "Action redone.", mailbox.names().Undo)()
}

func (mailbox *MailboxService) settled(failure error, result MailActionResult, message string, next string) func() {
	return FlowUtils{}.Either(HTTPUtils{}.Missed(failure, result.ActionID), func() {
		mailbox.toast(ToastRequested{Message: "Mail action failed."})
	}, func() { mailbox.applied(result, message, next) })
}

func (mailbox *MailboxService) applied(result MailActionResult, message string, next string) {
	mailbox.applicationEventPublisher.PublishEvent(MailActionApplied{ActionID: result.ActionID, ExpiresAt: result.ExpiresAt}, false)
	mailbox.applicationEventPublisher.PublishEvent(InboxRefreshRequested{Reason: "mail-action"}, false)
	mailbox.toast(MailboxUtils{}.UndoToast(message, next, result, time.Now()))
}

func (mailbox *MailboxService) reported(message string) {
	mailbox.toast(ToastRequested{Message: message})
	mailbox.applicationEventPublisher.PublishEvent(InboxRefreshRequested{Reason: "spam-report"}, false)
}

func (mailbox *MailboxService) toastAction(action string) func(string) {
	actions := map[string]func(string){mailbox.names().Undo: mailbox.undo, mailbox.names().Redo: mailbox.redo}
	if chosen, found := actions[action]; found {
		return chosen
	}
	return func(string) {}
}

func (mailbox *MailboxService) publishBadge(count int) {
	mailbox.applicationEventPublisher.PublishEvent(AppBadgeChanged{Count: count}, true)
}

func (mailbox *MailboxService) toast(request ToastRequested) {
	mailbox.applicationEventPublisher.PublishEvent(request, false)
}

// shown is true for the folders whose badge the user sees: the inbox, and
// every folder when "show unread for all folders" is on.
func (mailbox *MailboxService) shown(folder string) bool {
	return mailbox.ShowUnreadForAllFolders || folder == mailbox.names().Inbox
}

func (mailbox *MailboxService) badge(shown bool, count int) string {
	if !shown || count <= 0 {
		return ""
	}
	return strconv.Itoa(count)
}

func (mailbox *MailboxService) activityDue(timer string) bool {
	return timer == mailbox.names().Activity && mailbox.token != ""
}

func (mailbox *MailboxService) bearer() string {
	return "Bearer " + mailbox.token
}

func (mailbox *MailboxService) activityKey() string {
	return "caduceus_activity_viewed:" + mailbox.email
}

func (MailboxService) names() MailboxNames {
	return MailboxNames{Activity: "activity", Inbox: "inbox", LabelsKey: "caduceus_mailbox_names", Undo: "undo-mail-action", Redo: "redo-mail-action"}
}
