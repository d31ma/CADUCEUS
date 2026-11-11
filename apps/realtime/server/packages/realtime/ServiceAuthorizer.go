package realtime

type ServiceAuthorizer interface {
	Authorization(realm string) (string, error)
}
