package messaging

import "slices"

type ReactionNames struct{}

func (ReactionNames) All() []string {
	return []string{"thumbs_up", "thumbs_down", "heart", "laugh", "surprised", "sad", "party", "eyes", "check", "pray", "fire", "rocket"}
}

func (names ReactionNames) Check(name string) error {
	return names.refusal(slices.Contains(names.All(), name), name)
}

func (ReactionNames) refusal(known bool, name string) error {
	if known {
		return nil
	}
	return UnknownReaction{Name: name}
}
