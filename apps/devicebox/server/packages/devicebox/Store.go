package devicebox

type Store struct {
	documents Documents
}

func NewStore(documents Documents) Store {
	return Store{documents: documents}
}

func (store Store) EnsureDomain(domainId string) error {
	names := Names{}
	err := store.ensureAll(names.Slips(domainId), names.Bodies(domainId), names.Counts(domainId), names.Registry())
	if err != nil {
		return err
	}
	return store.register(names.Bodies(domainId), domainId)
}

func (store Store) SlipByEnvelope(domainId string, envelopeId string) (Slip, bool, error) {
	statement, err := SqlText{}.SlipByEnvelope(Names{}.Slips(domainId), envelopeId)
	if err != nil {
		return Slip{}, false, err
	}
	slips := []Slip{}
	err = store.documents.Rows(statement, &slips)
	return store.firstSlip(slips), len(slips) > 0, err
}

func (store Store) InsertBody(domainId string, body Body) (string, error) {
	return store.documents.Insert(Names{}.Bodies(domainId), body)
}

func (store Store) InsertSlip(domainId string, slip Slip) (string, error) {
	return store.documents.Insert(Names{}.Slips(domainId), slip)
}

func (store Store) Waiting(domainId string, deviceKey string, afterUs int64, limit int) ([]WaitingRow, error) {
	exists, err := store.HasDomain(domainId)
	if err != nil {
		return nil, err
	}
	return store.waitingIn(exists, domainId, deviceKey, afterUs, limit)
}

func (store Store) HasDomain(domainId string) (bool, error) {
	return store.documents.Exists(Names{}.Slips(domainId))
}

func (store Store) Body(domainId string, bodyId string) (Body, bool, error) {
	body := Body{}
	found, err := store.documents.Find(Names{}.Bodies(domainId), bodyId, &body)
	return body, found, err
}

func (store Store) SaveWaiting(domainId string, slip Slip) error {
	return store.documents.Patch(Names{}.Slips(domainId), slip.Id, waitingPatch{Waiting: slip.Waiting})
}

func (store Store) DeleteEnvelope(domainId string, slip Slip) error {
	err := store.documents.Delete(Names{}.Slips(domainId), slip.Id)
	if err != nil {
		return err
	}
	return store.documents.Delete(Names{}.Bodies(domainId), slip.BodyId)
}

func (store Store) DeleteBody(domainId string, bodyId string) error {
	return store.documents.Delete(Names{}.Bodies(domainId), bodyId)
}

func (store Store) Expired(domainId string, nowUs int64, limit int) ([]Slip, error) {
	statement, err := SqlText{}.ExpiredSlips(Names{}.Slips(domainId), nowUs, limit)
	if err != nil {
		return nil, err
	}
	slips := []Slip{}
	return slips, store.documents.Rows(statement, &slips)
}

func (store Store) ExpiredBodies(domainId string, nowUs int64, limit int) ([]Body, error) {
	statement, err := SqlText{}.ExpiredBodies(Names{}.Bodies(domainId), nowUs, limit)
	if err != nil {
		return nil, err
	}
	bodies := []Body{}
	return bodies, store.documents.Rows(statement, &bodies)
}

func (store Store) Domains() ([]RegistryRecord, error) {
	statement, err := SqlText{}.Registry(Names{}.Registry())
	if err != nil {
		return nil, err
	}
	records := []RegistryRecord{}
	err = store.documents.Read(Names{}.Registry(), func() error { return store.documents.Rows(statement, &records) })
	return records, err
}

func (store Store) waitingIn(exists bool, domainId string, deviceKey string, afterUs int64, limit int) ([]WaitingRow, error) {
	if !exists {
		return []WaitingRow{}, nil
	}
	statement, err := SqlText{}.WaitingFor(Names{}.Slips(domainId), deviceKey, afterUs, limit)
	if err != nil {
		return nil, err
	}
	rows := []WaitingRow{}
	return rows, store.documents.Rows(statement, &rows)
}

func (store Store) ensureAll(collections ...string) error {
	for _, collection := range collections {
		err := store.documents.Ensure(collection)
		if err != nil {
			return err
		}
	}
	return nil
}

func (store Store) register(bodies string, domainId string) error {
	statement, err := SqlText{}.RegistryRecord(Names{}.Registry(), bodies)
	if err != nil {
		return err
	}
	records := []RegistryRecord{}
	err = store.documents.Rows(statement, &records)
	if err != nil {
		return err
	}
	return store.addUnless(len(records) > 0, RegistryRecord{Name: bodies, DomainId: domainId})
}

func (store Store) addUnless(known bool, record RegistryRecord) error {
	if known {
		return nil
	}
	_, err := store.documents.Insert(Names{}.Registry(), record)
	return err
}

func (Store) firstSlip(slips []Slip) Slip {
	if len(slips) == 0 {
		return Slip{}
	}
	return slips[0]
}
