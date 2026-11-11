type AttachmentConflict struct {
	Field  string
	Reason string
}

func (failure *AttachmentConflict) Error() string {
	return failure.Field + ": " + failure.Reason
}
