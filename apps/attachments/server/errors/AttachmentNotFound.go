type AttachmentNotFound struct {
}

func (failure *AttachmentNotFound) Error() string {
	return "the attachment was not found"
}
