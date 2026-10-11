// ContactDeletion answers a delete: a CardDAV contact is pending until its
// mutation settles.
type ContactDeletion struct {
	Deleted    bool   `json:"deleted"`
	Pending    bool   `json:"pending"`
	MutationID string `json:"mutationId"`
	AccountID  string `json:"accountId"`
}
