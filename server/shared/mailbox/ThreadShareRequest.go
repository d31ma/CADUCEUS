// ThreadShareRequest asks for a new expiring link to the conversation of one
// message the owner may read; EmailId and ExpiresInDays are as the body sent
// them.
type ThreadShareRequest struct {
	OwnerEmail    string
	Domains       []string
	EmailId       any
	ExpiresInDays any
}
