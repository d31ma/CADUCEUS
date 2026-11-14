package outbox

type sameToken struct {
	source TokenSource
}

func (token sameToken) TokenFor(domainId string) (string, error) {
	return token.source.Token()
}
