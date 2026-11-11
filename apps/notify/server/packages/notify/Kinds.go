package notify

type Kinds struct{}

func (Kinds) Mention() string     { return "mention" }
func (Kinds) Direct() string      { return "dm" }
func (Kinds) ThreadReply() string { return "thread_reply" }
func (Kinds) Email() string       { return "email" }
