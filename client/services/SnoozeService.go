import (
	"encoding/json"
	"time"
)

// SnoozeService runs the snooze dialog of the open message: preset and
// custom times, the request, and what follows. On a message page it leaves
// the Undo to the shell (caduceus_pending_snooze_undo, then /inbox); beside
// the list it offers the Undo itself.
//
// @Service
type SnoozeService struct {
	// @Needs
	snoozeClient              *SnoozeClient
	secureRandom              SecureRandom
	applicationEventPublisher ApplicationEventPublisher

	Open   bool
	Busy   bool
	Custom string
	Error  string

	message OpenMessageChanged
	token   string
}

// @OnEvent({"replay":true})
func (service *SnoozeService) HearSession(changed SessionChanged) {
	service.token = changed.Token
}

// @OnEvent({"replay":true})
func (service *SnoozeService) HearMessage(opened OpenMessageChanged) {
	service.message = opened
}

// HearBackdrop closes the dialog on a click outside it.
// @OnEvent
func (service *SnoozeService) HearBackdrop(clicked BackdropClicked) {
	FlowUtils{}.When(clicked.Dialog == service.names().Dialog, service.Close)()
}

// HearSubmit snoozes until the custom date and time.
// @OnEvent
func (service *SnoozeService) HearSubmit(submitted FormSubmitted) {
	FlowUtils{}.When(submitted.Form == service.names().Dialog, service.submitCustom)()
	service.settle()
}

// HearToastAction undoes a snooze from its toast.
// @OnEvent
func (service *SnoozeService) HearToastAction(chosen ToastActionChosen) {
	FlowUtils{}.When(chosen.Action == service.names().UndoAction, func() { service.undo(chosen.Subject) })()
	service.settle()
}

// @OnEvent
func (service *SnoozeService) Settled(_ ViewSettled) {}

func (service *SnoozeService) OpenDialog() {
	service.Error = ""
	service.Open = LogicUtils{}.All(service.message.EmailId != "", service.message.Folder == "inbox")
}

func (service *SnoozeService) Close() {
	service.Open = service.Busy
}

// ChoosePreset snoozes until a preset such as "tomorrow".
func (service *SnoozeService) ChoosePreset(preset string, label string) {
	service.perform(SnoozePresetsUtils{}.Preset(preset, time.Now()), label)
}

func (SnoozeService) names() SnoozeNames {
	return SnoozeNames{Dialog: "reader-snooze", UndoAction: "reader-snooze-undo", PendingKey: "caduceus_pending_snooze_undo", Session: "session", Source: "snooze"}
}

func (service *SnoozeService) bearer() string {
	return "Bearer " + service.token
}

func (service *SnoozeService) settle() {
	service.applicationEventPublisher.PublishEvent(ViewSettled{Source: service.names().Source}, false)
}

func (service *SnoozeService) submitCustom() {
	moment := SnoozePresetsUtils{}.Custom(service.Custom, time.Now())
	service.Error = TextUtils{}.Choose(moment.After(time.Now()), "", "Choose a future date and time.")
	FlowUtils{}.When(service.Error == "", func() { service.perform(moment, "Custom") })()
}

func (service *SnoozeService) perform(until time.Time, label string) {
	requestID, _ := service.secureRandom.UUID()
	service.Busy = true
	service.Error = ""
	result, failure := service.snoozeClient.Snooze(service.bearer(), map[string]any{"emailId": service.message.EmailId, "until": SnoozePresetsUtils{}.Timestamp(until), "label": label, "clientRequestId": requestID})
	service.Busy = false
	subject := TextUtils{}.Choose(result.Count == 1, "conversation", TextUtils{}.Count(result.Count, "message", "messages"))
	message := "Snoozed " + subject + " until " + DateUtils{}.Label(SnoozePresetsUtils{}.Timestamp(until), time.Now())
	FlowUtils{}.Either(HTTPUtils{}.Missed(failure, result.SnoozeId), func() {
		service.Error = HTTPUtils{}.Message(failure, "Snooze failed.", "Network error.")
	}, func() { service.snoozed(result.SnoozeId, message) })()
}

// snoozed keeps the Undo for ten seconds and returns to the list.
func (service *SnoozeService) snoozed(snoozeID string, message string) {
	service.Open = false
	pending, _ := json.Marshal(PendingUndo{ID: snoozeID, Owner: service.message.Account, Message: message, ExpiresAt: time.Now().Add(10 * time.Second).UnixMilli()})
	service.applicationEventPublisher.PublishEvent(StorageWritten{Area: service.names().Session, Values: map[string]string{service.names().PendingKey: string(pending)}, Removed: []string{}}, false)
	service.applicationEventPublisher.PublishEvent(InboxRefreshRequested{Reason: service.names().Source}, false)
	FlowUtils{}.Either(service.message.Standalone, func() {
		service.applicationEventPublisher.PublishEvent(NavigationRequested{Target: "/inbox"}, false)
	}, func() {
		service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message, ActionLabel: "Undo", Action: service.names().UndoAction, Subject: snoozeID, DurationMs: 10000}, false)
		service.applicationEventPublisher.PublishEvent(MailClosed{EmailID: service.message.EmailId}, false)
	})()
}

func (service *SnoozeService) undo(snoozeID string) {
	service.applicationEventPublisher.PublishEvent(StorageWritten{Area: service.names().Session, Values: map[string]string{}, Removed: []string{service.names().PendingKey}}, false)
	_, failure := service.snoozeClient.Cancel(service.bearer(), snoozeID)
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: TextUtils{}.Choose(failure == nil, "Snooze undone.", "Could not undo snooze.")}, false)
	FlowUtils{}.When(failure == nil, func() {
		service.applicationEventPublisher.PublishEvent(InboxRefreshRequested{Reason: "snooze-undo"}, false)
	})()
}
