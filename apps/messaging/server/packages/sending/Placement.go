package sending

import (
	"slices"

	"mesail.dev/packages/core"
)

type Placement struct {
	Conversation core.Conversation
	Existing     []core.Participant
	Newcomers    []core.Participant
	Sender       core.Participant
}

func (placement Placement) all() []core.Participant {
	return slices.Concat(placement.Existing, placement.Newcomers, placement.unsavedSender())
}

func (placement Placement) unsavedSender() []core.Participant {
	if placement.Sender.Id != "" {
		return nil
	}
	return []core.Participant{placement.Sender}
}
