package attachments

type Missing struct{}

func (Missing) Error() string {
	return "the attachment was not found"
}
