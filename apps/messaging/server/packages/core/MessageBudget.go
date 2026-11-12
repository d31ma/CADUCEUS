package core

import "encoding/json"

type MessageBudget struct{}

func (MessageBudget) MaxContentBytes() int {
	return 384 << 10
}

func (budget MessageBudget) ContentBytes(body string, blocksJson string) int {
	return len(budget.encoded(body)) + len(budget.encoded(blocksJson)) + len(budget.encoded(budget.encoded(body)))
}

func (budget MessageBudget) Fits(body string, blocksJson string) bool {
	return budget.ContentBytes(body, blocksJson) <= budget.MaxContentBytes()
}

func (budget MessageBudget) Check(body string, blocksJson string) error {
	return budget.refusalUnless(budget.Fits(body, blocksJson))
}

func (budget MessageBudget) BlocksJson(blocks []Block) string {
	return budget.encodedBlocks(append([]Block{}, blocks...))
}

func (MessageBudget) refusalUnless(fits bool) error {
	if fits {
		return nil
	}
	return &TooLarge{Message: "the message is too large to store; send a shorter body"}
}

func (MessageBudget) encoded(text string) string {
	raw, _ := json.Marshal(text)
	return string(raw)
}

func (MessageBudget) encodedBlocks(blocks []Block) string {
	raw, _ := json.Marshal(blocks)
	return string(raw)
}
