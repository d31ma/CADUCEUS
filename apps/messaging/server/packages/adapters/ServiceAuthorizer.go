package adapters

type ServiceAuthorizer interface {
	Authorization(realm string) (string, error)
}
