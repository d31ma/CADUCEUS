import (
	"cmp"

	"mesail.dev/packages/ids"
	"mesail.dev/packages/notify"
	"mesail.dev/packages/webpush"
)

// @Service
type PushSenderService struct {
	// @Needs
	subscriptions *PushSubscriptionStoreService
}

func (service *PushSenderService) Send(record NotificationRecord, targets map[string]webpush.Target, deleteLimit int) (PushSendResult, error) {
	subscriptions, failure := service.subscriptions.ListForMember(record.DomainId, record.MemberId)
	result, problem := service.sendAll(record, service.usable(subscriptions, failure), targets, deleteLimit)
	return result, cmp.Or(failure, problem)
}

func (service *PushSenderService) usable(subscriptions []PushSubscriptionRecord, failure error) []PushSubscriptionRecord {
	if failure != nil {
		return []PushSubscriptionRecord{}
	}
	return subscriptions[:min(len(subscriptions), service.subscriptionLimit())]
}

func (service *PushSenderService) sendAll(record NotificationRecord, subscriptions []PushSubscriptionRecord, targets map[string]webpush.Target, deleteLimit int) (PushSendResult, error) {
	message := service.message(record)
	result := PushSendResult{Outcomes: []string{}}
	failures := []error{}
	for _, subscription := range subscriptions {
		outcome := service.sendOne(subscription, targets, message)
		removed, failure := service.removeGone(subscription, outcome, deleteLimit-result.Written)
		result.Outcomes, result.Written, failures = append(result.Outcomes, outcome), result.Written+removed, append(failures, failure)
	}
	return result, cmp.Or(failures...)
}

func (service *PushSenderService) sendOne(subscription PushSubscriptionRecord, targets map[string]webpush.Target, message webpush.Message) string {
	target, found := targets[subscription.Platform]
	if !found {
		return webpush.Outcomes{}.Refused()
	}
	outcome, _ := target.Send(webpush.Subscription{Endpoint: subscription.Endpoint, P256dh: subscription.P256dh, Auth: subscription.Auth}, message)
	return outcome
}

func (service *PushSenderService) removeGone(subscription PushSubscriptionRecord, outcome string, room int) (int, error) {
	if outcome != (webpush.Outcomes{}).Gone() || room < 1 {
		return 0, nil
	}
	return 1, service.subscriptions.Delete(subscription.DomainId, subscription.Id)
}

func (service *PushSenderService) message(record NotificationRecord) webpush.Message {
	title, body := notify.PushText{}.For(record.Kind, record.AuthorDisplayName)
	notificationId := ids.WireId{}.Add(service.prefix(), record.Id)
	return webpush.Message{
		Title:          title,
		Body:           body,
		Tag:            notificationId,
		Url:            service.conversationPath() + record.ConversationId,
		NotificationId: notificationId,
		Kind:           record.Kind,
		ConversationId: record.ConversationId,
		MessageId:      record.MessageId,
	}
}

func (service *PushSenderService) prefix() string {
	return "ntf"
}

func (service *PushSenderService) conversationPath() string {
	return "/conversations/"
}

func (service *PushSenderService) subscriptionLimit() int {
	return 10
}
