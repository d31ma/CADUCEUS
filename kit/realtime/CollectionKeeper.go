package realtime

import "mesail.dev/packages/collections"

type CollectionKeeper struct {
	Documents Documents
	Known     collections.KnownCollections
}

func (keeper CollectionKeeper) Exists(name string) (bool, error) {
	exists, err := keeper.Known.Exists(keeper.Documents, name)
	return exists, keeper.failure("collection check", err)
}

func (keeper CollectionKeeper) Ensure(name string) error {
	return keeper.failure("create collection", keeper.Known.Ensure(keeper.Documents, name))
}

func (keeper CollectionKeeper) Write(name string, write func() error) error {
	return keeper.Known.Write(keeper.Documents, name, write)
}

func (keeper CollectionKeeper) ReadFailure(name string, err error) error {
	return keeper.Known.ReadFailure(keeper.Documents, name, err)
}

func (CollectionKeeper) failure(operation string, err error) error {
	if err != nil {
		return Unavailable{Operation: operation}
	}
	return nil
}
