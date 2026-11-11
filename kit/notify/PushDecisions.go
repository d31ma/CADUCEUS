package notify

type PushDecisions struct{}

func (PushDecisions) Send() string             { return "send" }
func (PushDecisions) SkipActive() string       { return "skip_active" }
func (PushDecisions) SkipDoNotDisturb() string { return "skip_dnd" }
