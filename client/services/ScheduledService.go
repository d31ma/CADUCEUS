import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// @Service
type ScheduledService struct {
	// @Needs
	sendClient                *SendClient
	applicationEventPublisher ApplicationEventPublisher
	Items                     []ScheduledRow
	// @SessionState
	Loaded        bool
	SignedIn      bool
	authorization string
}

// @OnEvent({"replay":true})
func (scheduled *ScheduledService) SessionChanged(session SessionChanged) {
	scheduled.SignedIn = session.Token != ""
	scheduled.authorization = "Bearer " + session.Token
	scheduled.loadWhenSignedIn()
	scheduled.settle()
}

// Load reads the scheduled-send queue. A refused read keeps the list, a
// failed one empties it.
func (scheduled *ScheduledService) Load() {
	scheduled.Loaded = false
	found, failure := scheduled.sendClient.Scheduled(scheduled.authorization)
	scheduled.Items = scheduled.loadedRows(found, failure)
	scheduled.Loaded = true
}

// Edit loads the full message and hands it to the composer.
func (scheduled *ScheduledService) Edit(id string) {
	message, failure := scheduled.sendClient.ScheduledMessage(scheduled.authorization, id)
	scheduled.composeWhenLoaded(message, failure)
	scheduled.explainWhenFailed(failure)
}

// Confirmed cancels the scheduled send the person agreed to cancel.
//
// @OnEvent
func (scheduled *ScheduledService) Confirmed(confirmation ConfirmationGiven) {
	asked, failure := scheduled.cancelWhenAsked(confirmation)
	scheduled.Items = scheduled.withoutCancelled(confirmation.ID, asked, failure)
	scheduled.toastCancel(asked, failure)
	scheduled.settle()
}

func (scheduled *ScheduledService) loadWhenSignedIn() {
	if scheduled.SignedIn {
		scheduled.Load()
	}
}

func (scheduled *ScheduledService) loadedRows(found []ScheduledSend, failure error) []ScheduledRow {
	switch {
	case failure == nil:
		return scheduled.rows(found)
	case BehaviorErrorCode(failure) == scheduled.refused():
		return scheduled.Items
	default:
		return []ScheduledRow{}
	}
}

func (ScheduledService) refused() string { return "HTTP_STATUS" }

func (scheduled *ScheduledService) rows(found []ScheduledSend) []ScheduledRow {
	rows := []ScheduledRow{}
	for _, item := range found {
		rows = append(rows, ScheduledRow{
			ID: item.ID, Subject: item.Subject, Text: item.Text, LastError: item.LastError,
			RecipientLabel: strings.Join(scheduled.addresses(item.To), scheduled.listSeparator()),
			SendAtLabel:    DateLabelUtils{}.Numeric(item.SendAt),
			StatusLabel:    scheduled.statusLabel(item),
		})
	}
	return rows
}

func (ScheduledService) listSeparator() string { return ", " }

func (ScheduledService) statusLabel(item ScheduledSend) string {
	next := map[bool]string{true: "pending", false: DateLabelUtils{}.Numeric(item.NextAttempt)}[item.NextAttempt == ""]
	switch {
	case item.Status == "sending":
		return "Sending now"
	case item.Attempts > 0:
		return fmt.Sprintf("Retry %d · %s", item.Attempts, next)
	default:
		return "Queued"
	}
}

func (scheduled *ScheduledService) addresses(value any) []string {
	switch typed := value.(type) {
	case []any:
		return scheduled.texts(typed)
	case string:
		return []string{typed}
	default:
		return []string{}
	}
}

func (ScheduledService) texts(values []any) []string {
	texts := []string{}
	for _, value := range values {
		texts = append(texts, fmt.Sprint(value))
	}
	return texts
}

func (scheduled *ScheduledService) composeWhenLoaded(message ScheduledSend, failure error) {
	if failure == nil {
		scheduled.applicationEventPublisher.PublishEvent(scheduled.prefill(message), false)
	}
}

func (scheduled *ScheduledService) explainWhenFailed(failure error) {
	switch {
	case failure == nil:
		return
	case BehaviorErrorCode(failure) == scheduled.refused():
		scheduled.toast("This scheduled message is no longer editable.")
		scheduled.Load()
	default:
		scheduled.toast(scheduled.networkError())
	}
}

func (ScheduledService) networkError() string { return "Network error." }

func (scheduled *ScheduledService) prefill(message ScheduledSend) ComposeRequested {
	replyTo := append(scheduled.addresses(message.ReplyTo), "")
	return ComposeRequested{
		ScheduledID: message.ID, SendAt: message.SendAt, RemindAt: message.RemindAt, From: message.From,
		To: strings.Join(scheduled.addresses(message.To), scheduled.listSeparator()), Cc: strings.Join(scheduled.addresses(message.Cc), scheduled.listSeparator()),
		ReplyTo: replyTo[0], Subject: message.Subject, Body: cmp.Or(message.HTML, message.Text),
		InReplyTo: message.InReplyTo, References: message.References, Attachments: message.Attachments,
		OpenTracking: message.OpenTracking, LinkTracking: message.LinkTracking,
	}
}

func (scheduled *ScheduledService) cancelWhenAsked(confirmation ConfirmationGiven) (bool, error) {
	if confirmation.Action != scheduled.cancelAction() {
		return false, nil
	}
	_, failure := scheduled.sendClient.CancelScheduled(scheduled.authorization, confirmation.ID)
	return true, failure
}

func (ScheduledService) cancelAction() string { return "cancel-scheduled" }

func (scheduled *ScheduledService) withoutCancelled(id string, asked bool, failure error) []ScheduledRow {
	if !asked || failure != nil {
		return scheduled.Items
	}
	return slices.DeleteFunc(slices.Clone(scheduled.Items), func(row ScheduledRow) bool { return row.ID == id })
}

func (scheduled *ScheduledService) toastCancel(asked bool, failure error) {
	switch {
	case !asked:
		return
	case failure == nil:
		scheduled.toast("Scheduled send cancelled.")
	case BehaviorErrorCode(failure) == scheduled.refused():
		scheduled.toast("Failed to cancel.")
	default:
		scheduled.toast(scheduled.networkError())
	}
}

func (scheduled *ScheduledService) toast(message string) {
	scheduled.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (scheduled *ScheduledService) settle() {
	scheduled.applicationEventPublisher.PublishEvent(ViewSettled{Source: "scheduled"}, false)
}
