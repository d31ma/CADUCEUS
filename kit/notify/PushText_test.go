package notify

import "testing"

func pushText(kind string, author string) []string {
	title, body := PushText{}.For(kind, author)
	return []string{title, body}
}

func TestPushTextMention(t *testing.T) {
	expectSame(t, pushText("mention", "Ana"), []string{"New mention", "Ana mentioned you"})
}

func TestPushTextDirect(t *testing.T) {
	expectSame(t, pushText("dm", "Ana"), []string{"New direct message", "New direct message from Ana"})
}

func TestPushTextThreadReply(t *testing.T) {
	expectSame(t, pushText("thread_reply", "Ana"), []string{"New thread reply", "Ana replied in a thread"})
}

func TestPushTextEmail(t *testing.T) {
	expectSame(t, pushText("email", "Ana"), []string{"New email", "New email from Ana"})
}

func TestPushTextUnknownKind(t *testing.T) {
	expectSame(t, pushText("other", "Ana"), []string{"Mesail", "New notification from Ana"})
}

func TestPushTextWithoutAuthorName(t *testing.T) {
	expectSame(t, pushText("mention", ""), []string{"New mention", "someone mentioned you"})
}
