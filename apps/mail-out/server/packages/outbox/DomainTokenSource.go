package outbox

type DomainTokenSource interface {
	TokenFor(domainId string) (string, error)
}
