package notifications

type PushText struct {
	kinds Kinds
}

func (text PushText) For(kind string, authorDisplayName string) (string, string) {
	wording := text.wordingFor(kind)
	return wording.Title, wording.Before + text.name(authorDisplayName) + wording.After
}

func (text PushText) wordings() map[string]Wording {
	return map[string]Wording{
		text.kinds.Mention():     {Title: "New mention", After: " mentioned you"},
		text.kinds.Direct():      {Title: "New direct message", Before: "New direct message from "},
		text.kinds.ThreadReply(): {Title: "New thread reply", After: " replied in a thread"},
		text.kinds.Email():       {Title: "New email", Before: "New email from "},
	}
}

func (text PushText) wordingFor(kind string) Wording {
	wording, found := text.wordings()[kind]
	if !found {
		return Wording{Title: "Mesail", Before: "New notification from "}
	}
	return wording
}

func (PushText) name(authorDisplayName string) string {
	if authorDisplayName == "" {
		return "someone"
	}
	return authorDisplayName
}
