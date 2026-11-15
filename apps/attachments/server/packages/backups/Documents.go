package backups

type Documents interface {
	CollectionExists(name string) (bool, error)
	CreateCollection(name string) error
	Insert(collection string, document any) (string, error)
	Patch(collection string, id string, fields any, target any) (bool, error)
	DeleteById(collection string, id string) (bool, error)
}
