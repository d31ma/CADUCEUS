package sending

import "mesail.dev/packages/core"

type Newcomers struct{}

func (newcomers Newcomers) of(known []core.Participant, chosen []Recipient) []core.Participant {
	identities := newcomers.identities(known)
	added := []core.Participant{}
	for _, recipient := range chosen {
		added = append(added, newcomers.ifNew(recipient, Identities{}.ofRecipient(recipient), identities)...)
	}
	return added
}

func (newcomers Newcomers) external(added []core.Participant) bool {
	count := 0
	for _, participant := range added {
		count += Lists{}.oneIf(participant.Kind != (core.ParticipantKinds{}).Member())
	}
	return count > 0
}

func (Newcomers) identities(known []core.Participant) map[string]bool {
	identities := map[string]bool{}
	for _, participant := range known {
		identities[Identities{}.ofParticipant(participant)] = true
	}
	return identities
}

func (newcomers Newcomers) ifNew(recipient Recipient, identity string, known map[string]bool) []core.Participant {
	if known[identity] || recipient.Role == (core.RecipientRoles{}).Bcc() {
		return nil
	}
	known[identity] = true
	return []core.Participant{newcomers.participant(recipient)}
}

func (Newcomers) participant(recipient Recipient) core.Participant {
	return core.Participant{Kind: recipient.Kind, MemberId: recipient.MemberId, Address: recipient.Address, DisplayName: recipient.DisplayName, Role: recipient.Role}
}
