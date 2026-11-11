package channels

type Documents interface {
	Ensure(collection string) error
	Insert(collection string, document any) (string, error)
	Find(collection string, id string, target any) (bool, error)
	Replace(collection string, id string, document any) error
	All(collection string, where map[string]any, target any) error
}
