package reading

import (
	"encoding/json"

	"mesail.dev/packages/core"
)

type messageBlocks struct{}

func (blocks messageBlocks) of(message core.Message) ([]core.Block, error) {
	decoded := []core.Block{}
	err := json.Unmarshal([]byte(blocks.stored(message)), &decoded)
	return decoded, err
}

func (blocks messageBlocks) stored(message core.Message) string {
	if message.BlocksJson == "" || message.DeletedAt != "" {
		return blocks.none()
	}
	return message.BlocksJson
}

func (messageBlocks) encoded(blocks []core.Block) string {
	text, _ := json.Marshal(append([]core.Block{}, blocks...))
	return string(text)
}

func (messageBlocks) none() string {
	return "[]"
}
