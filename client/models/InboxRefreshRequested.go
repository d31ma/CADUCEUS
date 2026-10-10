// Something changed the mailbox; the list loads again.
type InboxRefreshRequested struct {
	Reason string `json:"reason"`
}
