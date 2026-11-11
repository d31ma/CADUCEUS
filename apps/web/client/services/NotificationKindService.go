// @Service
type NotificationKindService struct{}

func (kinds *NotificationKindService) Text(kind string) string {
	return kinds.lookup(kinds.texts(), kind, "New activity")
}

func (kinds *NotificationKindService) Icon(kind string) string {
	return kinds.lookup(kinds.icons(), kind, "bell")
}

func (kinds *NotificationKindService) PlaceWord(kind string) string {
	return kinds.lookup(kinds.placeWords(), kind, "in")
}

func (kinds *NotificationKindService) TabLabel(name string) string {
	return kinds.lookup(kinds.labels(), name, name)
}

func (kinds *NotificationKindService) TabNames() []string {
	return []string{kinds.AllTab(), kinds.mention(), kinds.direct(), kinds.threadReply(), kinds.email()}
}

func (kinds *NotificationKindService) AllTab() string {
	return "all"
}

func (kinds *NotificationKindService) texts() map[string]string {
	return map[string]string{
		kinds.mention():     "You were mentioned",
		kinds.direct():      "New direct message",
		kinds.threadReply(): "New reply in a thread",
		kinds.email():       "New email",
	}
}

func (kinds *NotificationKindService) icons() map[string]string {
	return map[string]string{
		kinds.mention():     "at",
		kinds.direct():      "chat",
		kinds.threadReply(): "thread",
		kinds.email():       "mail",
	}
}

func (kinds *NotificationKindService) placeWords() map[string]string {
	return map[string]string{
		kinds.direct(): kinds.fromWord(),
		kinds.email():  kinds.fromWord(),
	}
}

func (kinds *NotificationKindService) fromWord() string {
	return "from"
}

func (kinds *NotificationKindService) labels() map[string]string {
	return map[string]string{
		kinds.AllTab():      "All",
		kinds.mention():     "Mentions",
		kinds.direct():      "Direct messages",
		kinds.threadReply(): "Threads",
		kinds.email():       "Email",
	}
}

func (kinds *NotificationKindService) lookup(table map[string]string, key string, fallback string) string {
	text, found := table[key]
	if found {
		return text
	}
	return fallback
}

func (kinds *NotificationKindService) mention() string {
	return "mention"
}

func (kinds *NotificationKindService) direct() string {
	return "dm"
}

func (kinds *NotificationKindService) threadReply() string {
	return "thread_reply"
}

func (kinds *NotificationKindService) email() string {
	return "email"
}
