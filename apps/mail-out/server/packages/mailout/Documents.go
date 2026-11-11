package mailout

type Documents interface {
	CreateCollection(name string) error
	Insert(collection string, document any) (string, error)
	FindById(collection string, id string, target any) (bool, error)
	Patch(collection string, id string, fields any, target any) (bool, error)
	Command(command map[string]any) (any, error)
}
