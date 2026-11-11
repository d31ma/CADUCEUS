type ChangedRecipients struct {
	Message   string
	Addresses []string
}

func (failure *ChangedRecipients) Error() string {
	return failure.Message
}
