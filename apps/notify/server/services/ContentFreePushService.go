import (
	"cmp"
	"slices"
	"time"

	"mesail.dev/packages/notify"
	"mesail.dev/packages/webpush"
)

// @Service
type ContentFreePushService struct {
	// @Needs
	targets *PushTargetsService
	// @Needs
	presence *PresenceService
	// @Needs
	sender *PushSenderService
}

func (service *ContentFreePushService) NewMessage() string {
	return "chat.message"
}

func (service *ContentFreePushService) IncomingCall() string {
	return "call.incoming"
}

func (service *ContentFreePushService) MissedCall() string {
	return "call.missed"
}

func (service *ContentFreePushService) Push(push ContentFreePush) (CallPushReport, error) {
	targets, keysMissing := service.targets.ForPlatforms()
	presences, presenceFailure := service.presenceUnless(keysMissing, push.DomainId, push.PersonIds)
	usable := service.usable(targets, cmp.Or(keysMissing, presenceFailure))
	return service.sendAll(push, usable, presences)
}

func (service *ContentFreePushService) presenceUnless(keysMissing error, domainId string, people []string) (map[string]Presence, error) {
	if keysMissing != nil {
		return map[string]Presence{}, nil
	}
	return service.presence.States(domainId, people)
}

func (service *ContentFreePushService) usable(targets map[string]webpush.Target, failure error) map[string]webpush.Target {
	if failure != nil {
		return map[string]webpush.Target{}
	}
	return targets
}

func (service *ContentFreePushService) sendAll(push ContentFreePush, targets map[string]webpush.Target, presences map[string]Presence) (CallPushReport, error) {
	message := service.message(push)
	report := CallPushReport{}
	written := 0
	failures := []error{}
	for _, personId := range push.PersonIds {
		delivery := PushDelivery{DomainId: push.DomainId, MemberId: personId, Message: message}
		result, failure := service.sendOne(delivery, targets, presences[personId], service.writeLimit()-written)
		report, written, failures = service.counted(report, result), written+result.Written, append(failures, failure)
	}
	return report, cmp.Or(failures...)
}

func (service *ContentFreePushService) sendOne(delivery PushDelivery, targets map[string]webpush.Target, presence Presence, deleteLimit int) (PushSendResult, error) {
	if len(targets) == 0 || service.decision(presence) != (notify.PushDecisions{}).Send() {
		return PushSendResult{Outcomes: []string{}}, nil
	}
	return service.sender.SendMessage(delivery, targets, deleteLimit)
}

func (service *ContentFreePushService) decision(presence Presence) string {
	return notify.PushRules{}.Decide(presence.State, presence.DoNotDisturbUntil, time.Now())
}

func (service *ContentFreePushService) counted(report CallPushReport, result PushSendResult) CallPushReport {
	if slices.Contains(result.Outcomes, webpush.Outcomes{}.Delivered()) {
		return CallPushReport{Pushed: report.Pushed + 1, Skipped: report.Skipped}
	}
	return CallPushReport{Pushed: report.Pushed, Skipped: report.Skipped + 1}
}

func (service *ContentFreePushService) message(push ContentFreePush) webpush.Message {
	wording := service.wordingFor(push.Kind)
	return webpush.Message{
		Title: wording.Title,
		Body:  wording.Before + push.DomainId + wording.After,
		Tag:   push.DomainId + "/" + service.tagFor(push.Kind),
		Url:   service.chatPath(),
		Kind:  push.Kind,
	}
}

func (service *ContentFreePushService) wordingFor(kind string) notify.Wording {
	wording, found := service.wordings()[kind]
	if !found {
		return notify.Wording{Title: "Mesail", Before: "Open Mesail in ", After: service.sentenceEnd()}
	}
	return wording
}

func (service *ContentFreePushService) wordings() map[string]notify.Wording {
	return map[string]notify.Wording{
		service.NewMessage():   {Title: "New message", Before: "You have a new message in ", After: service.sentenceEnd()},
		service.IncomingCall(): {Title: "Incoming call", Before: "Someone is calling you in ", After: service.sentenceEnd()},
		service.MissedCall():   {Title: "Missed call", Before: "You missed a call in ", After: service.sentenceEnd()},
	}
}

func (service *ContentFreePushService) tagFor(kind string) string {
	if kind == service.NewMessage() {
		return "chat"
	}
	return "call"
}

func (service *ContentFreePushService) sentenceEnd() string {
	return "."
}

func (service *ContentFreePushService) chatPath() string {
	return "/conversations"
}

func (service *ContentFreePushService) writeLimit() int {
	return 20
}
