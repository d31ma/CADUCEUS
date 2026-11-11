import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// @Service
type ThreadRowService struct{}

func (rows *ThreadRowService) Row(message ThreadMessage) ThreadRow {
	name := rows.nameOf(message.Author)
	clock := rows.timeOf(message.CreatedAt)
	return ThreadRow{
		MessageId:  message.MessageId,
		Initials:   rows.initialsOf(name),
		AuthorName: name,
		Time:       clock,
		DateTime:   message.CreatedAt,
		Body:       message.Body,
		Edited:     message.EditedAt != "",
		Deleted:    message.DeletedAt != "",
		Reactions:  rows.reactionsOf(message.Reactions),
		Label:      rows.labelOf(name, clock),
	}
}

func (rows *ThreadRowService) Replies(messages []ThreadMessage, rootId string) []ThreadRow {
	return rows.repliesOf(messages, rootId)
}

func (rows *ThreadRowService) Root(messages []ThreadMessage, rootId string, known ThreadMessage) ThreadMessage {
	return rows.rootOf(messages, rootId, known)
}

func (rows *ThreadRowService) RepliesLabel(count int) string {
	return strconv.Itoa(count) + rows.space() + rows.repliesWord(count)
}

func (rows *ThreadRowService) repliesOf(messages []ThreadMessage, rootId string) []ThreadRow {
	replies := []ThreadRow{}
	for index := len(messages) - 1; index >= 0; index-- {
		replies = rows.appendReply(replies, messages[index], rootId)
	}
	return replies
}

func (rows *ThreadRowService) rootOf(messages []ThreadMessage, rootId string, known ThreadMessage) ThreadMessage {
	found := known
	for _, message := range messages {
		found = rows.matching(found, message, rootId)
	}
	return found
}

func (rows *ThreadRowService) appendReply(replies []ThreadRow, message ThreadMessage, rootId string) []ThreadRow {
	if message.MessageId == rootId {
		return replies
	}
	return append(replies, rows.Row(message))
}

func (rows *ThreadRowService) matching(found ThreadMessage, message ThreadMessage, rootId string) ThreadMessage {
	if message.MessageId == rootId {
		return message
	}
	return found
}

func (rows *ThreadRowService) repliesWord(count int) string {
	if count == 1 {
		return "reply"
	}
	return "replies"
}

func (rows *ThreadRowService) nameOf(author ThreadAuthor) string {
	if author.DisplayName == "" {
		return author.Address
	}
	return author.DisplayName
}

func (rows *ThreadRowService) timeOf(createdAt string) string {
	moment, failure := time.Parse(time.RFC3339, createdAt)
	if failure != nil {
		return ""
	}
	return moment.UTC().Format(rows.hourAndMinute())
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

func (rows *ThreadRowService) reactionsOf(reactions map[string]int) string {
	parts := []string{}
	for _, emoji := range rows.sortedEmoji(reactions) {
		parts = append(parts, emoji+rows.space()+strconv.Itoa(reactions[emoji]))
	}
	return strings.Join(parts, rows.separator())
}

func (rows *ThreadRowService) sortedEmoji(reactions map[string]int) []string {
	emoji := []string{}
	for each := range reactions {
		emoji = append(emoji, each)
	}
	sort.Strings(emoji)
	return emoji
}

func (rows *ThreadRowService) hourAndMinute() string {
	return "15:04"
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
