import (
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// NoticeUtils builds CADUCEUS' self-hosted equivalents of Mailspring's derived
// notices and orders them for the notification centre.
type NoticeUtils struct{}

// Build derives the notices from the shell's health state, highest priority
// first.
func (NoticeUtils) Build(state NoticeState) []Notice {
	notices := []Notice{}
	notices = NoticeUtils{}.offline(notices, state.Online)
	notices = NoticeUtils{}.update(notices, state.UpdateVersion)
	notices = NoticeUtils{}.accounts(notices, NoticeUtils{}.distinct(state.AccountErrors))
	notices = NoticeUtils{}.rules(notices, max(0, state.DisabledRuleCount))
	notices = NoticeUtils{}.push(notices, state.PushPrompt)
	return NoticeUtils{}.sorted(notices)
}

// Visible keeps every required notice and drops the dismissed optional ones.
func (NoticeUtils) Visible(notices []Notice, dismissed []string) []Notice {
	kept := slices.DeleteFunc(slices.Clone(notices), func(notice Notice) bool { return NoticeUtils{}.hidden(notice, dismissed) })
	return NoticeUtils{}.classed(kept)
}

// Dismissable is true when the notice with id may be dismissed.
func (NoticeUtils) Dismissable(notices []Notice, id string) bool {
	index := slices.IndexFunc(notices, func(notice Notice) bool { return notice.ID == id })
	return append([]Notice{{}}, notices...)[index+1].Dismissable
}

// PushPrompt offers push when the browser can ask, has not been asked yet and
// the user has no subscription.
func (NoticeUtils) PushPrompt(features BrowserFeatures, permission string, answered bool, subscriptions int) bool {
	return !slices.Contains([]bool{features.Notifications, !features.PushDisabled, permission == "default", answered, subscriptions == 0}, false)
}

// Label is the accessible name of the notification button.
func (NoticeUtils) Label(count int) string {
	return NoticeUtils{}.label(count)
}

func (NoticeUtils) offline(notices []Notice, online bool) []Notice {
	if online {
		return notices
	}
	return append(notices, NoticeUtils{}.identified(Notice{Kind: "offline", Priority: 5, Title: "CADUCEUS is offline.",
		Subtitle: "Mailbox actions will synchronize after the connection returns.",
		Actions:  []NoticeAction{{ID: "try-now", Label: "Try now"}}}))
}

func (NoticeUtils) update(notices []Notice, version string) []Notice {
	if version == "" {
		return notices
	}
	safe := regexp.MustCompile(`[^a-zA-Z0-9._:-]`).ReplaceAllString(version, "-")
	return append(notices, Notice{ID: "update-" + safe[:min(64, len(safe))], Kind: "update", Priority: 4,
		Title:    "A CADUCEUS update is ready (" + version[:min(80, len(version))] + ").",
		Subtitle: "Reload to use the latest locally installed web application.", Dismissable: true,
		Actions:  []NoticeAction{{ID: "reload-update", Label: "Reload"}}})
}

func (NoticeUtils) accounts(notices []Notice, errors []string) []Notice {
	if len(errors) == 0 {
		return notices
	}
	return append(notices, NoticeUtils{}.identified(Notice{Kind: "account-error", Priority: 3, Title: NoticeUtils{}.reconnecting(errors),
		Subtitle: "Stored mail remains available while CADUCEUS retries the live connection.",
		Actions:  []NoticeAction{{ID: "retry-accounts", Label: "Try again"}, {ID: "manage-accounts", Label: "Manage"}}}))
}

func (NoticeUtils) rules(notices []Notice, disabled int) []Notice {
	if disabled == 0 {
		return notices
	}
	return append(notices, NoticeUtils{}.identified(Notice{Kind: "disabled-rules", Priority: 2, Title: NoticeUtils{}.disabledRules(disabled),
		Actions: []NoticeAction{{ID: "view-rules", Label: "View mail rules"}}}))
}

func (NoticeUtils) push(notices []Notice, prompt bool) []Notice {
	if !prompt {
		return notices
	}
	return append(notices, NoticeUtils{}.identified(Notice{Kind: "push-prompt", Priority: 1, Title: "Enable new-mail notifications on this device?",
		Subtitle: "Notification privacy and account scope remain under your control.", Dismissable: true,
		Actions:  []NoticeAction{{ID: "notification-settings", Label: "Review settings"}}}))
}

// identified names a notice after its kind, for the kinds that occur once.
func (NoticeUtils) identified(notice Notice) Notice {
	notice.ID = notice.Kind
	return notice
}

func (NoticeUtils) reconnecting(errors []string) string {
	if len(errors) == 1 {
		return "Mail updates for " + errors[0] + " are reconnecting."
	}
	return strconv.Itoa(len(errors)) + " mailboxes are reconnecting."
}

func (NoticeUtils) disabledRules(count int) string {
	if count == 1 {
		return "One mail rule is disabled."
	}
	return strconv.Itoa(count) + " mail rules are disabled."
}

func (NoticeUtils) distinct(values []string) []string {
	kept := []string{}
	for _, value := range values {
		kept = append(kept, strings.TrimSpace(value))
	}
	kept = slices.DeleteFunc(kept, func(value string) bool { return value == "" })
	slices.Sort(kept)
	kept = slices.Compact(kept)
	return kept[:min(20, len(kept))]
}

func (NoticeUtils) sorted(notices []Notice) []Notice {
	sort.SliceStable(notices, func(left int, right int) bool { return NoticeUtils{}.before(notices[left], notices[right]) })
	return notices
}

func (NoticeUtils) before(left Notice, right Notice) bool {
	if left.Priority != right.Priority {
		return left.Priority > right.Priority
	}
	return left.ID < right.ID
}

func (NoticeUtils) hidden(notice Notice, dismissed []string) bool {
	return notice.Dismissable && slices.Contains(dismissed, notice.ID)
}

// classed gives each card its classes; the first card is the highest
// priority one.
func (NoticeUtils) classed(notices []Notice) []Notice {
	for index := range notices {
		notices[index].Classes = "shell-notification shell-notification-" + notices[index].Kind + map[bool]string{true: " highest-priority"}[index == 0]
	}
	return notices
}

func (NoticeUtils) label(count int) string {
	if count == 0 {
		return "Notifications, none active"
	}
	return "Notifications, " + strconv.Itoa(count) + " active"
}
