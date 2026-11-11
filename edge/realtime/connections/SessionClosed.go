package connections

type SessionClosed struct{}

func (SessionClosed) Error() string {
	return "session closed"
}
