import "encoding/json"

// SnoozeService runs the snooze dialog of the open message: preset and
// custom times, the request, and what happens after it (back to the list, an
// undo offer, or the offline queue).
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

	emailId    string
	folder     string
	standalone bool
	token      string
}

// @OnEvent({"replay":true})
func (service *SnoozeService) SessionChanged(changed SessionChanged) {
	service.token = changed.Token
}

// @OnEvent({"replay":true})
func (service *SnoozeService) MessageOpened(opened OpenMessageChanged) {
	service.emailId = opened.EmailId
	service.folder = opened.Folder
	service.standalone = opened.Standalone
}

func (service *SnoozeService) OpenDialog() {
	service.open()
}

func (SnoozeService) backdrop() string { return "snooze-backdrop" }

func (service *SnoozeService) Close() {
	service.close(service.backdrop())
}

func (service *SnoozeService) Dismiss(className string) {
	service.close(className)
}

func (service *SnoozeService) ChoosePreset(until string, label string, untilLabel string) {
	service.perform(SnoozeChoice{Until: until, Label: label, UntilLabel: untilLabel})
}

func (service *SnoozeService) SubmitCustom(until string, untilLabel string) {
	service.Error = TextUtils{}.Choose(until == "", "Choose a future date and time.", "")
	service.performWhen(until != "", SnoozeChoice{Until: until, Label: "Custom", UntilLabel: untilLabel})
}

func (SnoozeService) inboxFolder() string { return "inbox" }

func (service *SnoozeService) open() {
	if service.emailId != "" && service.folder == service.inboxFolder() {
		service.Error = ""
		service.Open = true
	}
}

func (service *SnoozeService) close(className string) {
	if !service.Busy && className == service.backdrop() {
		service.Open = false
	}
}

func (service *SnoozeService) performWhen(valid bool, choice SnoozeChoice) {
	if valid {
		service.perform(choice)
	}
}

func (SnoozeService) snoozeKind() string { return "snooze" }

func (service *SnoozeService) perform(choice SnoozeChoice) {
	requestId, _ := service.secureRandom.UUID()
	body := map[string]any{"emailId": service.emailId, "until": choice.Until, "label": choice.Label, "clientRequestId": requestId}
	service.Busy = true
	service.Error = ""
	result, failure := service.snoozeClient.Snooze(body, "Bearer "+service.token)
	encoded, _ := json.Marshal(body)
	service.finished(result, FailureUtils{}.Code(failure), OfflineRequestRequested{Id: requestId, Kind: service.snoozeKind(), Path: "/snooze", Method: "POST", Body: string(encoded), Message: "Snooze until " + choice.UntilLabel, CoalesceKey: "snooze:" + service.emailId, EmailId: service.emailId})
	service.Busy = false
}

func (service *SnoozeService) finished(result SnoozeResult, code string, offline OfflineRequestRequested) {
	switch code {
	case "":
		service.snoozed(result, TextUtils{}.Choose(result.Count == 1, "conversation", TextUtils{}.Count(result.Count, "message", "messages")), offline.Message)
	case FailureUtils{}.OfflineCode():
		service.queued(offline)
	default:
		service.Error = "Snooze failed."
	}
}

func (service *SnoozeService) snoozed(result SnoozeResult, subject string, offlineMessage string) {
	service.Open = false
	service.applicationEventPublisher.PublishEvent(SnoozeCompleted{SnoozeId: result.SnoozeId, Message: "Snoozed " + subject + " " + offlineMessage[len("Snooze "):], Standalone: service.standalone}, false)
	service.leave(service.standalone)
	service.applicationEventPublisher.PublishEvent(InboxRefreshRequested{Reason: service.snoozeKind()}, false)
}

func (service *SnoozeService) leave(standalone bool) {
	if standalone {
		service.applicationEventPublisher.PublishEvent(NavigationRequested{Target: service.inboxFolder()}, false)
		return
	}
	service.applicationEventPublisher.PublishEvent(MailBackRequested{Reason: service.snoozeKind()}, false)
}

func (service *SnoozeService) queued(offline OfflineRequestRequested) {
	service.Open = false
	service.applicationEventPublisher.PublishEvent(offline, false)
	service.applicationEventPublisher.PublishEvent(NavigationRequested{Target: service.inboxFolder()}, false)
}
