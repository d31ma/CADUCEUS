package collections

type CollectionStore interface {
	CollectionExists(name string) (bool, error)
	CreateCollection(name string) error
}
