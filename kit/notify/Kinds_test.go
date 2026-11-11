package notify

import "testing"

func TestKindValues(t *testing.T) {
	kinds := Kinds{}
	expectSame(t, []string{kinds.Mention(), kinds.Direct(), kinds.ThreadReply(), kinds.Email()}, []string{"mention", "dm", "thread_reply", "email"})
}

func TestPushDecisionValues(t *testing.T) {
	decisions := PushDecisions{}
	expectSame(t, []string{decisions.Send(), decisions.SkipActive(), decisions.SkipDoNotDisturb()}, []string{"send", "skip_active", "skip_dnd"})
}
