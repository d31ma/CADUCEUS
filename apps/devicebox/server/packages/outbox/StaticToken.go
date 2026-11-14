package outbox

type StaticToken struct {
	Value string
}

func (token StaticToken) Token() (string, error) {
	return token.Value, nil
}
