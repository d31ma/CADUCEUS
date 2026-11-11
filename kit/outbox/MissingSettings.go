package outbox

type MissingSettings struct{}

func (MissingSettings) Error() string {
	return "the application JSON has no \"outbox\" settings"
}
