package inbound

import "mesail.dev/packages/core"

type Roster struct {
	participants core.ParticipantStore
}

func NewRoster(participants core.ParticipantStore) Roster {
	return Roster{participants: participants}
}

func (roster Roster) Ensure(domainId string, conversationId string, wanted []core.Participant) error {
	existing, err := roster.participants.OfConversation(domainId, conversationId)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	missing := roster.missing(roster.identities(existing), wanted)
	err = roster.insert(missing[:min(len(missing), roster.maximumAdded())])
	if err != nil {
		return Unavailability{}.Of(err)
	}
	return nil
}

func (Roster) maximumAdded() int {
	return 20
}

func (roster Roster) identities(existing []core.Participant) map[string]bool {
	known := map[string]bool{}
	for _, participant := range existing {
		known[roster.identity(participant)] = true
	}
	return known
}

func (roster Roster) missing(known map[string]bool, wanted []core.Participant) []core.Participant {
	missing := []core.Participant{}
	for _, participant := range wanted {
		missing = append(missing, roster.ifNew(known, roster.identity(participant), participant)...)
	}
	return missing
}

func (Roster) ifNew(known map[string]bool, identity string, participant core.Participant) []core.Participant {
	if known[identity] {
		return nil
	}
	known[identity] = true
	return []core.Participant{participant}
}

func (roster Roster) identity(participant core.Participant) string {
	if participant.Kind == (core.ParticipantKinds{}).Member() {
		return participant.Kind + roster.separator() + participant.MemberId
	}
	return roster.addressKind() + roster.separator() + Addresses{}.Normal(participant.Address)
}

func (Roster) separator() string {
	return "\n"
}

func (Roster) addressKind() string {
	return "address"
}

func (roster Roster) insert(missing []core.Participant) error {
	for _, participant := range missing {
		_, err := roster.participants.Insert(participant)
		if err != nil {
			return err
		}
	}
	return nil
}
