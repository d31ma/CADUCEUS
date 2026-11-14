package realtime

type ServiceTokenSource interface {
	Authorization(domainId string) (string, error)
}
