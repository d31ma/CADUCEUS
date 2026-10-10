// An undoable mail action the server applied; the shell offers Undo.
type MailActionApplied struct {
	ActionID  string        `json:"actionId"`
	Message   string        `json:"message"`
	ExpiresAt string        `json:"expiresAt"`
	Emails    []EmailChange `json:"emails"`
}
