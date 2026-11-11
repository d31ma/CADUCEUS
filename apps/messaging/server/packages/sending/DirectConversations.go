package sending

import (
	"maps"
	"slices"
	"strings"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
	kit "mesail.dev/packages/recipients"
)

type DirectConversations struct {
	stores core.Stores
	now    time.Time
}

func NewDirectConversations(stores core.Stores, now time.Time) DirectConversations {
	return DirectConversations{stores: stores, now: now}
}

func (direct DirectConversations) pick(caller oidc.Caller, request core.SendRequest, chosen []Recipient) (Placement, error) {
	sender := direct.senderParticipant(caller)
	added := Newcomers{}.of([]core.Participant{sender}, chosen)
	return direct.route(direct.chatOnly(chosen))(caller, request, sender, added)
}

func (direct DirectConversations) route(chatOnly bool) func(oidc.Caller, core.SendRequest, core.Participant, []core.Participant) (Placement, error) {
	if chatOnly {
		return direct.chat
	}
	return direct.email
}

func (direct DirectConversations) chat(caller oidc.Caller, request core.SendRequest, sender core.Participant, added []core.Participant) (Placement, error) {
	key := Hashes{}.participantKey(direct.memberIds(sender, added))
	stored, found, err := direct.stores.Conversations.ByParticipantKey(caller.DomainId, key)
	if err != nil {
		return Placement{}, Failures{}.store(err)
	}
	conversation := direct.either(found, stored, direct.conversation(caller, key, ""))
	return direct.reuseRoute(found)(caller, conversation, sender, added)
}

func (direct DirectConversations) email(caller oidc.Caller, request core.SendRequest, sender core.Participant, added []core.Participant) (Placement, error) {
	subject := strings.TrimSpace(request.Email.Subject)
	conversation := direct.conversation(caller, "", subject)
	conversation.HasExternal = Newcomers{}.external(added)
	return Placement{Conversation: conversation, Newcomers: added, Sender: sender}, Failures{}.subjectRefusal(subject)
}

func (direct DirectConversations) reuseRoute(found bool) func(oidc.Caller, core.Conversation, core.Participant, []core.Participant) (Placement, error) {
	if found {
		return direct.reuse
	}
	return direct.create
}

func (direct DirectConversations) reuse(caller oidc.Caller, stored core.Conversation, sender core.Participant, added []core.Participant) (Placement, error) {
	conversation, participant, err := core.NewAccess(direct.stores).Postable(caller, core.ConversationIds{}.Wire(stored))
	if err != nil {
		return Placement{}, err
	}
	existing, err := direct.stores.Participants.OfConversation(caller.DomainId, core.ConversationIds{}.Wire(conversation))
	if err != nil {
		return Placement{}, Failures{}.store(err)
	}
	return Placement{Conversation: conversation, Existing: existing, Sender: participant}, nil
}

func (DirectConversations) create(caller oidc.Caller, fresh core.Conversation, sender core.Participant, added []core.Participant) (Placement, error) {
	return Placement{Conversation: fresh, Newcomers: added, Sender: sender}, nil
}

func (DirectConversations) either(found bool, stored core.Conversation, fresh core.Conversation) core.Conversation {
	if found {
		return stored
	}
	return fresh
}

func (direct DirectConversations) chatOnly(chosen []Recipient) bool {
	count := 0
	for _, recipient := range chosen {
		count += Lists{}.oneIf(recipient.Delivery != string(kit.Deliveries{}.Chat()))
	}
	return count == 0
}

func (DirectConversations) memberIds(sender core.Participant, added []core.Participant) []string {
	unique := map[string]bool{sender.MemberId: true}
	for _, participant := range added {
		unique[participant.MemberId] = true
	}
	return slices.Sorted(maps.Keys(unique))
}

func (direct DirectConversations) conversation(caller oidc.Caller, participantKey string, subject string) core.Conversation {
	return core.Conversation{
		DomainId:       caller.DomainId,
		Kind:           core.ConversationKinds{}.Direct(),
		Visibility:     core.Visibilities{}.Private(),
		Subject:        subject,
		ParticipantKey: participantKey,
		CreatedMonth:   core.MessageIds{}.Month(direct.now),
		CreatedAt:      Months{}.stamp(direct.now),
	}
}

func (DirectConversations) senderParticipant(caller oidc.Caller) core.Participant {
	return core.Participant{
		DomainId:    caller.DomainId,
		Kind:        core.ParticipantKinds{}.Member(),
		MemberId:    caller.MemberId,
		Address:     Lists{}.address(caller.Email),
		DisplayName: caller.DisplayName,
		Role:        core.RecipientRoles{}.To(),
	}
}
