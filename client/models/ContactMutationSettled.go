// ContactMutationSettled is the outcome of a CardDAV change: "applied",
// "conflict", "failed", or "queued" when it did not settle in time.
type ContactMutationSettled struct {
	ContactID string `json:"contactId"`
	Kind      string `json:"kind"`
	State     string `json:"state"`
	Conflict  string `json:"conflict"`
}
