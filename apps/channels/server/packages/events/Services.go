package events

type Services struct{}

func (Services) Identity() string    { return "identity" }
func (Services) Directory() string   { return "directory" }
func (Services) Channels() string    { return "channels" }
func (Services) Messaging() string   { return "messaging" }
func (Services) Attachments() string { return "attachments" }
func (Services) MailIn() string      { return "mail-in" }
func (Services) MailOut() string     { return "mail-out" }
func (Services) Search() string      { return "search" }
func (Services) Realtime() string    { return "realtime" }
func (Services) Notify() string      { return "notify" }
