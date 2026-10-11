// ContactGroupsChanged says a group was created, renamed or deleted.
// DeletedID names a deleted group.
type ContactGroupsChanged struct {
	DeletedID string `json:"deletedId"`
}
