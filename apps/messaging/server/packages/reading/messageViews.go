package reading

import (
	"errors"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type messageViews struct {
	viewer     oidc.Caller
	reactions  reactionCounts
	deliveries deliveryViews
}

func (views messageViews) all(messages []core.Message) ([]core.MessageView, error) {
	built := []core.MessageView{}
	failures := []error{}
	for _, message := range messages {
		view, err := views.one(message)
		built = append(built, view)
		failures = append(failures, err)
	}
	return built, errors.Join(failures...)
}

func (views messageViews) one(message core.Message) (core.MessageView, error) {
	messageId := core.MessageIds{}.Wire(message.Month, message.Id)
	blocks, blocksErr := messageBlocks{}.of(message)
	deliveries, deliveriesErr := views.deliveries.of(views.viewer, message)
	return core.MessageView{
		Message:     message,
		Blocks:      blocks,
		Reactions:   views.reactions.of(messageId),
		MyReactions: views.reactions.mine(messageId, views.viewer.MemberId),
		Deliveries:  deliveries,
	}, errors.Join(blocksErr, deliveriesErr)
}
