package inbound

import (
	"encoding/json"
	"strings"

	"mesail.dev/packages/core"
	"mesail.dev/packages/richtext"
)

type EmailBudget struct{}

func (EmailBudget) PlainTextBytes() int {
	return 64 << 10
}

func (EmailBudget) LastResortTextBytes() int {
	return 16 << 10
}

func (budget EmailBudget) Applied(message core.Message) core.Message {
	return budget.lastResort(budget.fitted(message))
}

func (budget EmailBudget) fitted(message core.Message) core.Message {
	if budget.fits(message) {
		return message
	}
	return budget.cut(message, budget.PlainTextBytes())
}

func (budget EmailBudget) lastResort(message core.Message) core.Message {
	if budget.fits(message) {
		return message
	}
	return budget.cut(message, budget.LastResortTextBytes())
}

func (budget EmailBudget) cut(message core.Message, limit int) core.Message {
	message.Body = strings.ToValidUTF8(message.Body[:min(len(message.Body), limit)], "")
	message.BlocksJson = budget.blocksOf(message.Body)
	message.BodyFormat = core.BodyFormats{}.Text()
	message.Truncated = true
	return message
}

func (EmailBudget) fits(message core.Message) bool {
	return core.MessageBudget{}.Fits(message.Body, message.BlocksJson)
}

func (EmailBudget) blocksOf(text string) string {
	raw, _ := json.Marshal(richtext.Converter{}.FromText(text).Blocks)
	return string(raw)
}
