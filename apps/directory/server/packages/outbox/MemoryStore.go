package outbox

type MemoryStore struct {
	documents map[Ref]Fields
	order     []Ref
	failures  map[string]error
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{documents: map[Ref]Fields{}, order: []Ref{}, failures: map[string]error{}}
}

func (store *MemoryStore) Put(ref Ref, fields Fields) {
	store.write(ref, fields)
}

func (store *MemoryStore) Fields(ref Ref) Fields {
	return store.documents[ref]
}

func (store *MemoryStore) FailNextSave(err error) {
	store.failures[store.saveOperation()] = err
}

func (store *MemoryStore) FailNextLoad(err error) {
	store.failures[store.loadOperation()] = err
}

func (store *MemoryStore) FailNextPending(err error) {
	store.failures[store.pendingOperation()] = err
}

func (store *MemoryStore) Pending(limit int) ([]Document, error) {
	err := store.takeFailure(store.pendingOperation())
	if err != nil {
		return nil, err
	}
	refs := store.pendingRefs()
	return store.documentsOf(refs[:max(0, min(limit, len(refs)))]), nil
}

func (store *MemoryStore) Load(ref Ref) (Fields, error) {
	err := store.takeFailure(store.loadOperation())
	if err != nil {
		return Fields{}, err
	}
	return store.find(ref)
}

func (store *MemoryStore) Save(ref Ref, fields Fields) error {
	err := store.takeFailure(store.saveOperation())
	if err != nil {
		return err
	}
	store.write(ref, fields)
	return nil
}

func (store *MemoryStore) takeFailure(operation string) error {
	err := store.failures[operation]
	delete(store.failures, operation)
	return err
}

func (store *MemoryStore) pendingRefs() []Ref {
	refs := []Ref{}
	for _, ref := range store.order {
		refs = append(refs, store.ifPending(ref)...)
	}
	return refs
}

func (store *MemoryStore) ifPending(ref Ref) []Ref {
	if !store.documents[ref].OutboxWaiting {
		return []Ref{}
	}
	return []Ref{ref}
}

func (store *MemoryStore) documentsOf(refs []Ref) []Document {
	documents := []Document{}
	for _, ref := range refs {
		documents = append(documents, Document{Ref: ref, Fields: store.documents[ref]})
	}
	return documents
}

func (store *MemoryStore) find(ref Ref) (Fields, error) {
	fields, found := store.documents[ref]
	if !found {
		return Fields{}, NotFound{Kind: store.documentKind(), Key: ref.Collection + "/" + ref.Id}
	}
	return fields, nil
}

func (store *MemoryStore) write(ref Ref, fields Fields) {
	_, known := store.documents[ref]
	if !known {
		store.order = append(store.order, ref)
	}
	store.documents[ref] = fields
}

func (MemoryStore) saveOperation() string {
	return "save"
}

func (MemoryStore) loadOperation() string {
	return "load"
}

func (MemoryStore) pendingOperation() string {
	return "pending"
}

func (MemoryStore) documentKind() string {
	return "document"
}
