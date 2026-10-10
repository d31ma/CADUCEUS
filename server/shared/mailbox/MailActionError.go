// MailActionError is a refused or failed mail action with the HTTP status
// it answers: 400 by default, 404, 409 or 503.
type MailActionError struct {
	Message string
	Status  int
}

func (failure *MailActionError) Error() string {
	return failure.Message
}
