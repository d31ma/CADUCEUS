package outbox

type TokenSource interface {
	Token() (string, error)
}
