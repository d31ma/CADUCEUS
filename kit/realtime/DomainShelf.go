package realtime

import "mesail.dev/packages/collections"

type DomainShelf struct {
	Storage Storage
	Base    string
}

func (shelf DomainShelf) Find(domainId string, where map[string]any, target any) (bool, error) {
	exists, err := shelf.exists(domainId)
	if err != nil || !exists {
		return false, err
	}
	found, err := shelf.documents().First(shelf.collection(domainId), where, target)
	return found, shelf.failure(err)
}

func (shelf DomainShelf) All(domainId string, where map[string]any, target any) error {
	exists, err := shelf.exists(domainId)
	if err != nil || !exists {
		return err
	}
	return shelf.failure(shelf.documents().Every(shelf.collection(domainId), where, target))
}

func (shelf DomainShelf) Insert(domainId string, document any) error {
	return shelf.write(domainId, func(collection string) error {
		_, err := shelf.Storage.Documents.Insert(collection, document)
		return err
	})
}

func (shelf DomainShelf) Patch(domainId string, id string, fields any) error {
	return shelf.write(domainId, func(collection string) error {
		_, err := shelf.Storage.Documents.Patch(collection, id, fields, nil)
		return err
	})
}

func (shelf DomainShelf) Delete(domainId string, id string) error {
	return shelf.write(domainId, func(collection string) error {
		_, err := shelf.Storage.Documents.DeleteById(collection, id)
		return err
	})
}

func (shelf DomainShelf) write(domainId string, change func(collection string) error) error {
	collection := shelf.collection(domainId)
	return shelf.failure(shelf.keeper().Write(collection, func() error { return change(collection) }))
}

func (shelf DomainShelf) exists(domainId string) (bool, error) {
	return shelf.keeper().Exists(shelf.collection(domainId))
}

func (shelf DomainShelf) failure(err error) error {
	if err != nil {
		return Unavailable{Operation: shelf.Base}
	}
	return nil
}

func (shelf DomainShelf) documents() DocumentShelf {
	return DocumentShelf{Storage: shelf.Storage}
}

func (shelf DomainShelf) keeper() CollectionKeeper {
	return CollectionKeeper{Documents: shelf.Storage.Documents}
}

func (shelf DomainShelf) collection(domainId string) string {
	return collections.CollectionName{}.ForDomain(shelf.Base, domainId)
}
