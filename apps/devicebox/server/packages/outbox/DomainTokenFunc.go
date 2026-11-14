package outbox

type DomainTokenFunc func(domainId string) (string, error)

func (tokens DomainTokenFunc) TokenFor(domainId string) (string, error) {
	return tokens(domainId)
}
