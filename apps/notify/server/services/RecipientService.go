import (
	"slices"

	"mesail.dev/packages/events"
	"mesail.dev/packages/notify"
)

// @Service
type RecipientService struct {
	// @Needs
	settings *MemberSettingsStoreService
}

func (service *RecipientService) For(domainId string, payload events.MessageCreated, participants []string) (FanoutBatch, error) {
	settings, failure := service.settings.ForMembers(domainId, service.candidates(payload, participants))
	muted := service.muted(settings, payload.ConversationId)
	recipients := notify.RecipientRules{}.For(payload, participants, muted)
	return FanoutBatch{DomainId: domainId, Message: payload, Recipients: recipients, Settings: settings}, failure
}

func (service *RecipientService) Resume(domainId string, payload events.MessageCreated, recipients []notify.Recipient) (FanoutBatch, error) {
	settings, failure := service.settings.ForMembers(domainId, service.memberIds(recipients))
	return FanoutBatch{DomainId: domainId, Message: payload, Recipients: recipients, Settings: settings}, failure
}

func (service *RecipientService) muted(settings map[string]MemberSettingsRecord, conversationId string) notify.MutedSet {
	muted := notify.MutedSet{}
	for memberId, record := range settings {
		muted[memberId] = slices.Contains(record.MutedConversationIds, conversationId)
	}
	return muted
}

func (service *RecipientService) candidates(payload events.MessageCreated, participants []string) []string {
	readers := events.ReaderLists{}.ForConversation(payload.ConversationKind, payload.MemberIds)
	return slices.Concat(payload.MentionedMemberIds, readers, participants)
}

func (service *RecipientService) memberIds(recipients []notify.Recipient) []string {
	memberIds := []string{}
	for _, recipient := range recipients {
		memberIds = append(memberIds, recipient.MemberId)
	}
	return memberIds
}
