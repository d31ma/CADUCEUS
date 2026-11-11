package realtime

import "errors"

type failingWrites struct {
	*memoryStorage
}

func (failingWrites) writeFailure() error {
	return errors.New("writes are down")
}

func (documents failingWrites) Insert(collection string, document any) (string, error) {
	return "", documents.writeFailure()
}

func (documents failingWrites) Patch(collection string, id string, fields any, target any) (bool, error) {
	return false, documents.writeFailure()
}

func (documents failingWrites) DeleteById(collection string, id string) (bool, error) {
	return false, documents.writeFailure()
}

func (documents failingWrites) CreateCollection(name string) error {
	return documents.writeFailure()
}
