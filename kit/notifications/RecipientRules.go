package notifications

import (
	"slices"
	"strings"

	"mesail.dev/packages/events"
)

type RecipientRules struct {
	kinds Kinds
}

func (rules RecipientRules) For(message events.MessageCreated, threadMemberIds []string, muted MutedSet) []Recipient {
	candidates := rules.withKind(message.MentionedMemberIds, rules.kinds.Mention())
	candidates = append(candidates, rules.withKind(message.MemberIds, rules.conversationKind(message))...)
	candidates = append(candidates, rules.withKind(threadMemberIds, rules.threadKind(message.ThreadRootId))...)
	candidates = rules.withoutEmptyKind(candidates)
	candidates = rules.onlyReaders(candidates, rules.readers(message))
	candidates = rules.firstPerMember(candidates)
	candidates = rules.withoutMuted(candidates, muted)
	return rules.byMemberId(candidates)
}

func (RecipientRules) directConversation() string {
	return "direct"
}

func (RecipientRules) withKind(memberIds []string, kind string) []Recipient {
	recipients := []Recipient{}
	for _, memberId := range memberIds {
		recipients = append(recipients, Recipient{MemberId: memberId, Kind: kind})
	}
	return recipients
}

func (rules RecipientRules) conversationKind(message events.MessageCreated) string {
	if message.ConversationKind != rules.directConversation() {
		return ""
	}
	return rules.directKinds()[message.IsEmail]
}

func (rules RecipientRules) directKinds() map[bool]string {
	return map[bool]string{
		true:  rules.kinds.Email(),
		false: rules.kinds.Direct(),
	}
}

func (rules RecipientRules) threadKind(threadRootId string) string {
	if threadRootId == "" {
		return ""
	}
	return rules.kinds.ThreadReply()
}

func (RecipientRules) readers(message events.MessageCreated) map[string]bool {
	readers := map[string]bool{}
	for _, memberId := range message.MemberIds {
		readers[memberId] = true
	}
	delete(readers, message.AuthorMemberId)
	return readers
}

func (RecipientRules) withoutEmptyKind(candidates []Recipient) []Recipient {
	return slices.DeleteFunc(candidates, func(candidate Recipient) bool {
		return candidate.Kind == ""
	})
}

func (RecipientRules) onlyReaders(candidates []Recipient, readers map[string]bool) []Recipient {
	return slices.DeleteFunc(candidates, func(candidate Recipient) bool {
		return !readers[candidate.MemberId]
	})
}

func (RecipientRules) firstPerMember(candidates []Recipient) []Recipient {
	seen := map[string]bool{}
	return slices.DeleteFunc(candidates, func(candidate Recipient) bool {
		repeated := seen[candidate.MemberId]
		seen[candidate.MemberId] = true
		return repeated
	})
}

func (rules RecipientRules) withoutMuted(candidates []Recipient, muted MutedSet) []Recipient {
	return slices.DeleteFunc(candidates, func(candidate Recipient) bool {
		return muted[candidate.MemberId] && candidate.Kind != rules.kinds.Mention()
	})
}

func (RecipientRules) byMemberId(candidates []Recipient) []Recipient {
	slices.SortFunc(candidates, func(first Recipient, second Recipient) int {
		return strings.Compare(first.MemberId, second.MemberId)
	})
	return candidates
}
