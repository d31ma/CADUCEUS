import (
	"slices"
	"strconv"
	"strings"
	"time"
)

// ShareUtils prepares a shared conversation for the public share page.
type ShareUtils struct{}

// Token is the link token of a /share/<token> address, or empty.
func (ShareUtils) Token(path string, token string) string {
	return map[bool]string{true: token}[strings.HasPrefix(path, "/share/")]
}

// Unavailable is the page's error text: none for an available conversation.
func (ShareUtils) Unavailable(available bool, token string) string {
	return ShareUtils{}.unavailable(available, token == "")
}

// Decorated adds the labels the view shows and wraps each HTML body in a
// locked-down document.
func (ShareUtils) Decorated(thread SharedThread, now time.Time) SharedThread {
	thread.ExpiresAtLabel = DateLabelUtils{}.Stamp(thread.ExpiresAt, now)
	thread.MessageCount = strconv.Itoa(len(thread.Messages)) + map[bool]string{true: " message", false: " messages"}[len(thread.Messages) == 1]
	thread.Messages = ShareUtils{}.messages(thread.Messages, now)
	return thread
}

// Filename is the name of one attachment of the conversation.
func (ShareUtils) Filename(thread SharedThread, id string) string {
	return TextUtils{}.FirstOf(ShareUtils{}.attachment(thread, id).Filename, "attachment")
}

func (ShareUtils) unavailable(available bool, missing bool) string {
	switch {
	case available:
		return ""
	case missing:
		return "This shared conversation is unavailable."
	}
	return "This shared conversation is unavailable. It may have expired or been revoked."
}

func (ShareUtils) messages(messages []SharedMessage, now time.Time) []SharedMessage {
	for index := range messages {
		messages[index] = ShareUtils{}.message(messages[index], now)
	}
	return messages
}

func (ShareUtils) message(message SharedMessage, now time.Time) SharedMessage {
	message.Initial = strings.ToUpper(string([]rune(message.From)[:min(1, len([]rune(message.From)))]))
	separator := ", "
	message.ToLabel = strings.Join(message.To, separator)
	message.CcLabel = strings.Join(message.Cc, separator)
	message.ReceivedAtLabel = DateLabelUtils{}.Stamp(message.ReceivedAt, now)
	message.ProtectedHTML = ProtectedMailUtils{}.Document(message.HTMLBody)
	message.Attachments = ShareUtils{}.sized(append([]SharedAttachment{}, message.Attachments...))
	return message
}

func (ShareUtils) sized(attachments []SharedAttachment) []SharedAttachment {
	for index := range attachments {
		attachments[index].SizeLabel = SizeUtils{}.Label(attachments[index].Size)
	}
	return attachments
}

func (ShareUtils) attachment(thread SharedThread, id string) SharedAttachment {
	all := []SharedAttachment{}
	for _, message := range thread.Messages {
		all = append(all, message.Attachments...)
	}
	index := slices.IndexFunc(all, func(attachment SharedAttachment) bool { return attachment.ID == id })
	return append([]SharedAttachment{{}}, all...)[index+1]
}
