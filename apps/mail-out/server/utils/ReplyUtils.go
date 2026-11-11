import "slices"

type ReplyUtils struct{}

func (ReplyUtils) MailboxGone(enhanced string) bool {
	return slices.Contains([]string{"5.1.0", "5.1.1", "5.1.10"}, enhanced)
}
