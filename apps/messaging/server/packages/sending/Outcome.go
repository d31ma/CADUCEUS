package sending

import (
	"encoding/json"

	"mesail.dev/packages/core"
)

type Outcome struct{}

func (Outcome) of(message core.Message) (core.SendOutcome, error) {
	blocks := []core.Block{}
	err := json.Unmarshal([]byte(message.BlocksJson), &blocks)
	if err != nil {
		return core.SendOutcome{}, err
	}
	rows, err := core.DeliveryCodec{}.Decode(message.Deliveries)
	if err != nil {
		return core.SendOutcome{}, err
	}
	view := core.MessageView{Message: message, Blocks: blocks, Reactions: map[string]int{}, MyReactions: []string{}, Deliveries: rows}
	return core.SendOutcome{Message: view, Deliveries: rows}, nil
}
