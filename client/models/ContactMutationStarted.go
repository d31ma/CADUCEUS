// ContactMutationStarted is a queued CardDAV change to follow until it
// settles. Kind is "save" or "delete"; Conflict is the message to show when
// the server kept its own version.
type ContactMutationStarted struct {
	ContactID  string `json:"contactId"`
	MutationID string `json:"mutationId"`
	AccountID  string `json:"accountId"`
	Kind       string `json:"kind"`
	Conflict   string `json:"conflict"`
}
