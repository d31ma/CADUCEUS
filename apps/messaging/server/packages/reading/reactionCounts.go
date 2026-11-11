package reading

import (
	"slices"

	"mesail.dev/packages/core"
)

type reactionCounts struct {
	rows []core.Reaction
}

func (counts reactionCounts) of(messageId string) map[string]int {
	totals := map[string]int{}
	for _, row := range counts.active(messageId) {
		totals[row.Emoji]++
	}
	return totals
}

func (counts reactionCounts) mine(messageId string, memberId string) []string {
	emojis := []string{}
	for _, row := range counts.active(messageId) {
		emojis = append(emojis, counts.ifMine(row, memberId)...)
	}
	slices.Sort(emojis)
	return emojis
}

func (counts reactionCounts) active(messageId string) []core.Reaction {
	kept := []core.Reaction{}
	for _, row := range counts.rows {
		kept = append(kept, counts.ifActive(row, messageId)...)
	}
	return kept
}

func (reactionCounts) ifActive(row core.Reaction, messageId string) []core.Reaction {
	if row.Active && row.MessageId == messageId {
		return []core.Reaction{row}
	}
	return nil
}

func (reactionCounts) ifMine(row core.Reaction, memberId string) []string {
	if row.MemberId == memberId {
		return []string{row.Emoji}
	}
	return nil
}
