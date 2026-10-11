import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"
)

// MailboxUtils holds the sidebar's pure mailbox logic: the badge tally, the
// personal mailbox labels and the texts of mailbox-action toasts.
type MailboxUtils struct{}

// Tally counts summaries per folder, per domain for hosted mail and per
// connected account, and the unread inbox messages.
func (MailboxUtils) Tally(summaries []MailSummary) FolderCountsChanged {
	counted := FolderCountsChanged{Counts: map[string]int{}, ByDomain: map[string]map[string]int{}, ByAccount: map[string]map[string]int{}, UnreadInboxByDomain: map[string]int{}}
	return MailboxUtils{}.tallied(counted, summaries)
}

// Names reads the cached labels and keeps those of the signed-in domains.
func (MailboxUtils) Names(text string, domains []string) map[string]string {
	names := map[string]string{}
	json.Unmarshal([]byte(text), &names)
	return MailboxUtils{}.Allowed(names, domains)
}

// Allowed keeps the trimmed labels of the signed-in domains, keyed in lower case.
func (MailboxUtils) Allowed(names map[string]string, domains []string) map[string]string {
	return MailboxUtils{}.allowed(names, MailboxUtils{}.allowedDomains(domains))
}

// Encoded is the JSON that the session keeps for the labels.
func (MailboxUtils) Encoded(names map[string]string) string {
	text, _ := json.Marshal(names)
	return string(text)
}

// WithActivity copies counts with a new Activity badge.
func (MailboxUtils) WithActivity(counts map[string]int, activity int) map[string]int {
	copied := MailboxUtils{}.copied(counts)
	copied["activity"] = activity
	return copied
}

// MovedMessage is the toast text after a drop on a folder.
func (MailboxUtils) MovedMessage(count int, folder string, domain string) string {
	return "Moved " + MailboxUtils{}.messages(count) + " to " + MailboxUtils{}.place(folder, domain) + "."
}

// UndoToast offers the undo (or redo) action of a mailbox action until the
// server's undo window closes, for at most ten seconds; a closed window shows
// the plain message.
func (MailboxUtils) UndoToast(message string, action string, result MailActionResult, now time.Time) ToastRequested {
	return MailboxUtils{}.undoToast(ToastRequested{Message: message, ActionLabel: MailboxUtils{}.label(action), Action: action, Subject: result.ActionID}, MailboxUtils{}.remaining(result.ExpiresAt, now))
}

// SpamDone is the toast text after a spam report.
func (MailboxUtils) SpamDone(spam bool) string {
	return MailboxUtils{}.either(spam, "Reported spam.", "Moved to inbox.")
}

// SpamFailure is the toast text when a spam report fails.
func (MailboxUtils) SpamFailure(spam bool) string {
	return MailboxUtils{}.either(spam, "Report spam failed.", "Not spam failed.")
}

// BadgeCount is the installed-app badge: every unread inbox message, or one
// signed-in domain's when the user chose that domain.
func (MailboxUtils) BadgeCount(counted FolderCountsChanged, preference BadgePreference, domains []string) int {
	domain := strings.ToLower(strings.TrimSpace(preference.Domain))
	focused := !slices.Contains([]bool{preference.Scope == "domain", slices.Contains(MailboxUtils{}.allowedDomains(domains), domain)}, false)
	return max(0, map[bool]int{true: counted.UnreadInboxByDomain[domain], false: counted.UnreadInbox}[focused])
}

// NotificationAction is the mailbox patch and toast of a notification
// button, or no patch for an unknown button.
func (MailboxUtils) NotificationAction(action string) (map[string]any, string) {
	patches := map[string]map[string]any{"mark-read": {"read": true}, "archive": {"folder": "archive"}}
	messages := map[string]string{"mark-read": "Marked read from notification.", "archive": "Archived from notification."}
	return patches[action], messages[action]
}

// SafeID accepts the message identifiers a notification may carry.
func (MailboxUtils) SafeID(id string) bool {
	return TokenUtils{}.Fits(id, TokenUtils{}.Identifier()+"._:-", TokenBounds{Shortest: 1, Longest: 200})
}

func (MailboxUtils) allowedDomains(domains []string) []string {
	return strings.Fields(strings.ToLower(strings.Join(domains, " ")))
}

func (MailboxUtils) tallied(counted FolderCountsChanged, summaries []MailSummary) FolderCountsChanged {
	for _, summary := range summaries {
		MailboxUtils{}.count(&counted, summary, TextUtils{}.FirstOf(summary.SystemFolder, TextUtils{}.FirstOf(summary.Folder, MailboxUtils{}.inbox())))
	}
	return counted
}

func (MailboxUtils) count(counted *FolderCountsChanged, summary MailSummary, folder string) {
	domain := TextUtils{}.FirstOf(strings.ToLower(summary.Domain), "unknown")
	counted.Counts[folder]++
	tables, key := MailboxUtils{}.scope(counted, summary.ProviderAccountID, domain)
	MailboxUtils{}.ensure(tables, key)[folder]++
	all, hosted := MailboxUtils{}.unread(folder, summary.Read, summary.ProviderAccountID)
	counted.UnreadInbox += all
	counted.UnreadInboxByDomain[domain] += hosted
}

func (MailboxUtils) scope(counted *FolderCountsChanged, account string, domain string) (map[string]map[string]int, string) {
	if account != "" {
		return counted.ByAccount, account
	}
	return counted.ByDomain, domain
}

func (MailboxUtils) ensure(tables map[string]map[string]int, key string) map[string]int {
	if _, found := tables[key]; !found {
		tables[key] = map[string]int{}
	}
	return tables[key]
}

// unread counts an unread inbox message for the whole badge, and for its
// domain's badge when it is hosted mail.
func (MailboxUtils) unread(folder string, read bool, account string) (int, int) {
	all := map[bool]int{true: 1}[folder == MailboxUtils{}.inbox() && !read]
	return all, all * map[bool]int{true: 1}[account == ""]
}

func (MailboxUtils) allowed(names map[string]string, domains []string) map[string]string {
	kept := map[string]string{}
	for domain, name := range names {
		kept[strings.ToLower(domain)] = map[bool]string{true: strings.TrimSpace(name)}[slices.Contains(domains, strings.ToLower(domain))]
	}
	return MailboxUtils{}.withoutEmpty(kept)
}

func (MailboxUtils) withoutEmpty(names map[string]string) map[string]string {
	for domain, name := range names {
		delete(names, map[bool]string{true: domain}[name == ""])
	}
	delete(names, "")
	return names
}

func (MailboxUtils) copied(counts map[string]int) map[string]int {
	copied := map[string]int{}
	for folder, count := range counts {
		copied[folder] = count
	}
	return copied
}

func (MailboxUtils) messages(count int) string {
	if count == 1 {
		return "message"
	}
	return strconv.Itoa(count) + " messages"
}

func (MailboxUtils) place(folder string, domain string) string {
	if domain != "" {
		return folder + " (" + domain + ")"
	}
	return folder
}

func (MailboxUtils) remaining(expiresAt string, now time.Time) int {
	window := MailboxUtils{}.undoWindow()
	expires, failure := time.Parse(time.RFC3339Nano, expiresAt)
	if failure != nil {
		return window
	}
	return int(min(int64(window), max(0, expires.Sub(now).Milliseconds())))
}

func (MailboxUtils) undoToast(toast ToastRequested, remaining int) ToastRequested {
	if remaining <= 0 {
		return ToastRequested{Message: toast.Message, DurationMs: 2500}
	}
	toast.DurationMs = remaining
	return toast
}

func (MailboxUtils) label(action string) string {
	return MailboxUtils{}.either(action == "undo-mail-action", "Undo", "Redo")
}

func (MailboxUtils) either(yes bool, chosen string, other string) string {
	if yes {
		return chosen
	}
	return other
}

func (MailboxUtils) inbox() string { return "inbox" }

func (MailboxUtils) undoWindow() int { return 10000 }
