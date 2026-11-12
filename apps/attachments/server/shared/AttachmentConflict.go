type AttachmentConflict struct {
	Field       string
	Reason      string
	ErrorNumber int
}

func (failure *AttachmentConflict) Error() string {
	return failure.Field + ": " + failure.Reason
}

func (failure *AttachmentConflict) Number() int {
	return failure.ErrorNumber
}
