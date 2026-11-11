package reading

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/messaging"
)

type reactionRules struct{}

func (rules reactionRules) check(reaction string) error {
	return rules.refusal(messaging.ReactionNames{}.Check(reaction))
}

func (reactionRules) refusal(failure error) error {
	if failure != nil {
		return &core.InvalidRequest{Message: "the reaction is not a known reaction name", Field: "reaction"}
	}
	return nil
}
