// ConversationArchiveError is a conversation archive that cannot go ahead,
// with the HTTP status it answers (404 or 409).
type ConversationArchiveError struct {
	Message string
	Status  int
}

func (failure *ConversationArchiveError) Error() string {
	return failure.Message
}
