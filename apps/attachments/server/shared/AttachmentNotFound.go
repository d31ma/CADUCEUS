import "cmp"

type AttachmentNotFound struct {
	Message     string
	ErrorNumber int
}

func (failure *AttachmentNotFound) Error() string {
	return cmp.Or(failure.Message, "the attachment was not found")
}

func (failure *AttachmentNotFound) Number() int {
	return cmp.Or(failure.ErrorNumber, 3507)
}
