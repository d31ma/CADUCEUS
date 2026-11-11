package realtime

type CollectionKeeper struct {
	Documents Documents
}

func (keeper CollectionKeeper) Exists(name string) (bool, error) {
	exists, err := keeper.Documents.CollectionExists(name)
	if err != nil {
		return false, Unavailable{Operation: "collection check"}
	}
	return exists, nil
}

func (keeper CollectionKeeper) Ensure(name string) error {
	exists, err := keeper.Exists(name)
	if err != nil {
		return err
	}
	return keeper.create(name, exists)
}

func (keeper CollectionKeeper) create(name string, exists bool) error {
	if exists {
		return nil
	}
	return keeper.settled(name, keeper.Documents.CreateCollection(name))
}

func (keeper CollectionKeeper) settled(name string, failure error) error {
	exists, err := keeper.Documents.CollectionExists(name)
	if failure != nil && (err != nil || !exists) {
		return Unavailable{Operation: "create collection"}
	}
	return nil
}
