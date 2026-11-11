package channels

type Store struct {
	documents Documents
	relay     Relay
	registry  Registry
}

func NewStore(documents Documents, relay Relay) Store {
	return Store{documents: documents, relay: relay, registry: NewRegistry(documents)}
}

func (store Store) InsertChannel(document ChannelDocument) (string, error) {
	return store.insert(Names{}.Channels(document.DomainId), document)
}

func (store Store) SaveChannel(document ChannelDocument) error {
	id := document.Id
	document.Id = ""
	return store.replace(Names{}.Channels(document.DomainId), id, document)
}

func (store Store) FindChannel(domainId string, id string) (ChannelDocument, bool, error) {
	document := ChannelDocument{}
	found, err := store.find(Names{}.Channels(domainId), id, &document)
	document.Id = id
	return document, store.sameDomain(found, document, domainId), err
}

func (store Store) ChannelNamed(domainId string, name string) (bool, error) {
	channels := []ChannelDocument{}
	err := store.matching(Names{}.Channels(domainId), map[string]any{store.nameField(): name}, &channels)
	return len(channels) > 0, err
}

func (store Store) Channels(domainId string) ([]ChannelDocument, error) {
	channels := []ChannelDocument{}
	err := store.matching(Names{}.Channels(domainId), map[string]any{}, &channels)
	return channels, err
}

func (store Store) FindMembership(domainId string, channelId string, memberId string) (MembershipDocument, bool, error) {
	memberships := []MembershipDocument{}
	where := map[string]any{store.channelField(): channelId, store.memberField(): memberId}
	err := store.matching(Names{}.Memberships(domainId), where, &memberships)
	return store.first(memberships), len(memberships) > 0, err
}

func (store Store) MembershipsOf(domainId string, memberId string) ([]MembershipDocument, error) {
	memberships := []MembershipDocument{}
	err := store.matching(Names{}.Memberships(domainId), map[string]any{store.memberField(): memberId}, &memberships)
	return memberships, err
}

func (store Store) InsertMembership(document MembershipDocument) error {
	_, err := store.insert(Names{}.Memberships(document.DomainId), document)
	return err
}

func (store Store) SaveMembership(document MembershipDocument) error {
	id := document.Id
	document.Id = ""
	return store.replace(Names{}.Memberships(document.DomainId), id, document)
}

func (store Store) matching(collection string, where map[string]any, target any) error {
	err := store.documents.Ensure(collection)
	if err != nil {
		return err
	}
	return store.documents.All(collection, where, target)
}

func (store Store) find(collection string, id string, target any) (bool, error) {
	err := store.documents.Ensure(collection)
	if err != nil {
		return false, err
	}
	return store.documents.Find(collection, id, target)
}

func (store Store) insert(collection string, document any) (string, error) {
	err := store.documents.Ensure(collection)
	if err != nil {
		return "", err
	}
	err = store.registry.Register(collection)
	if err != nil {
		return "", err
	}
	id, err := store.documents.Insert(collection, document)
	if err != nil {
		return "", err
	}
	store.relay.Drain(collection, id)
	return id, nil
}

func (store Store) replace(collection string, id string, document any) error {
	err := store.documents.Replace(collection, id, document)
	if err != nil {
		return err
	}
	store.relay.Drain(collection, id)
	return nil
}

func (Store) sameDomain(found bool, document ChannelDocument, domainId string) bool {
	return found && document.DomainId == domainId
}

func (Store) first(memberships []MembershipDocument) MembershipDocument {
	if len(memberships) == 0 {
		return MembershipDocument{}
	}
	return memberships[0]
}

func (Store) nameField() string {
	return "name"
}

func (Store) channelField() string {
	return "channelId"
}

func (Store) memberField() string {
	return "memberId"
}
