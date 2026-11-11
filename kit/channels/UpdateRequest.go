package channels

type UpdateRequest struct {
	Topic        *string
	EmailAddress *string
	Archived     *bool
}
