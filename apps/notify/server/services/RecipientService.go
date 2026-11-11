import (
	"slices"

	"mesail.dev/packages/events"
	"mesail.dev/packages/notifications"
)

// @Service
type RecipientService struct {
	// @Needs
	settings *MemberSettingsStoreService
}

func (service *RecipientService) For(domainId string, payload events.MessageCreated, participants []string) (FanoutBatch, error) {
	settings, failure := service.settings.ForMembers(domainId, payload.MemberIds)
	muted := service.muted(settings, payload.ConversationId)
	recipients := notifications.RecipientRules{}.For(payload, participants, muted)
	return FanoutBatch{DomainId: domainId, Message: payload, Recipients: recipients, Settings: settings}, failure
}

func (service *RecipientService) Resume(domainId string, payload events.MessageCreated, recipients []notifications.Recipient) (FanoutBatch, error) {
	settings, failure := service.settings.ForMembers(domainId, payload.MemberIds)
	return FanoutBatch{DomainId: domainId, Message: payload, Recipients: recipients, Settings: settings}, failure
}

func (service *RecipientService) muted(settings map[string]MemberSettingsRecord, conversationId string) notifications.MutedSet {
	muted := notifications.MutedSet{}
	for memberId, record := range settings {
		muted[memberId] = slices.Contains(record.MutedConversationIds, conversationId)
	}
	return muted
}
