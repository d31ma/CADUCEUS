import (
	"strconv"
	"strings"
)

// @Service
type ThreadRowService struct {
	// @Needs
	clock Clock
	// @Needs
	times *ConversationTimeService
	// @Needs
	reactions *ConversationReactionService
}

func (rows *ThreadRowService) Row(message Message) ThreadRow {
	name := rows.nameOf(message.Author)
	clock := rows.times.Time(message.CreatedAt, rows.zone())
	return ThreadRow{
		MessageId:  message.MessageId,
		Initials:   rows.initialsOf(name),
		AuthorName: name,
		Time:       clock,
		DateTime:   message.CreatedAt,
		Body:       message.Body,
		Blocks:     message.Blocks,
		Truncated:  message.Truncated,
		Edited:     message.EditedAt != "",
		Deleted:    message.DeletedAt != "",
		Reactions:  rows.reactions.Reactions(message.Reactions, message.MyReactions),
		Label:      rows.labelOf(name, clock),
	}
}

func (rows *ThreadRowService) Timeline(root Message, messages []Message) []ThreadRow {
	return append(rows.rootRows(root), rows.oldestFirst(messages)...)
}

func (rows *ThreadRowService) Replies(messages []Message) []ThreadRow {
	return rows.oldestFirst(messages)
}

func (rows *ThreadRowService) RepliesLabel(root Message, shown int) string {
	count := max(shown, root.ReplyCount)
	return strconv.Itoa(count) + rows.space() + rows.repliesWord(count)
}

func (rows *ThreadRowService) PendingRow(name string, body string) ThreadRow {
	return rows.Row(Message{Author: Author{DisplayName: name}, Body: body})
}

func (rows *ThreadRowService) rootRows(root Message) []ThreadRow {
	if root.MessageId == "" {
		return []ThreadRow{}
	}
	row := rows.Row(root)
	row.IsRoot = true
	return []ThreadRow{row}
}

func (rows *ThreadRowService) oldestFirst(messages []Message) []ThreadRow {
	replies := []ThreadRow{}
	for index := len(messages) - 1; index >= 0; index-- {
		replies = append(replies, rows.Row(messages[index]))
	}
	return replies
}

func (rows *ThreadRowService) repliesWord(count int) string {
	if count == 1 {
		return "reply"
	}
	return "replies"
}

func (rows *ThreadRowService) nameOf(author Author) string {
	if author.DisplayName == "" {
		return author.Address
	}
	return author.DisplayName
}

func (rows *ThreadRowService) labelOf(name string, clock string) string {
	if clock == "" {
		return name
	}
	return name + rows.separator() + clock
}

func (rows *ThreadRowService) initialsOf(name string) string {
	initials := ""
	for _, word := range rows.firstWords(name) {
		initials += strings.ToUpper(string([]rune(word)[0]))
	}
	return initials
}

func (rows *ThreadRowService) firstWords(name string) []string {
	words := strings.Fields(name)
	return words[:min(len(words), rows.initialsLength())]
}

func (rows *ThreadRowService) initialsLength() int {
	return 2
}

func (rows *ThreadRowService) space() string {
	return " "
}

func (rows *ThreadRowService) separator() string {
	return ", "
}

func (rows *ThreadRowService) zone() string {
	zone, _ := rows.clock.TimeZone()
	return zone
}
