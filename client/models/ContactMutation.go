// ContactMutation is the state of a queued CardDAV change: "applied",
// "conflict" or "failed" once settled.
type ContactMutation struct {
	ContactID string `json:"contactId"`
	State     string `json:"state"`
}
