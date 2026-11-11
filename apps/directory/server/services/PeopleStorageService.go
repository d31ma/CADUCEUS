import "slices"

// @Service
type PeopleStorageService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (storage *PeopleStorageService) ByPersonId(personId string) (PersonDocument, bool, error) {
	return storage.findWhere(storage.personField(), personId)
}

func (storage *PeopleStorageService) All() ([]PersonDocument, error) {
	people := []PersonDocument{}
	failure := storage.documents.All(storage.names.People(), map[string]any{}, &people)
	return people, failure
}

func (storage *PeopleStorageService) InDomain(domainId string) ([]PersonDocument, error) {
	people, failure := storage.All()
	return storage.membersOf(people, domainId), failure
}

func (storage *PeopleStorageService) Leaving() ([]PersonDocument, error) {
	people := []PersonDocument{}
	failure := storage.documents.All(storage.names.People(), map[string]any{storage.stateField(): storage.deleted()}, &people)
	return storage.withDomains(people), failure
}

func (storage *PeopleStorageService) Store(person PersonDocument) (PersonDocument, error) {
	person.DomainIds = storage.list(person.DomainIds)
	person.AdminDomainIds = storage.list(person.AdminDomainIds)
	return storage.insertOrReplace(person)
}

func (storage *PeopleStorageService) findWhere(field string, value string) (PersonDocument, bool, error) {
	person := PersonDocument{}
	found, failure := storage.documents.FindOne(storage.names.People(), map[string]any{field: value}, &person)
	return person, found, failure
}

func (storage *PeopleStorageService) insertOrReplace(person PersonDocument) (PersonDocument, error) {
	if person.Id == "" {
		id, failure := storage.documents.Insert(storage.names.People(), person)
		person.Id = id
		return person, failure
	}
	saved := person
	saved.Id = ""
	return person, storage.documents.Replace(storage.names.People(), person.Id, saved)
}

func (storage *PeopleStorageService) list(values []string) []string {
	return append([]string{}, values...)
}

func (storage *PeopleStorageService) membersOf(people []PersonDocument, domainId string) []PersonDocument {
	return slices.DeleteFunc(people, func(person PersonDocument) bool { return !slices.Contains(person.DomainIds, domainId) })
}

func (storage *PeopleStorageService) withDomains(people []PersonDocument) []PersonDocument {
	return slices.DeleteFunc(people, func(person PersonDocument) bool { return len(person.DomainIds) == 0 })
}

func (storage *PeopleStorageService) stateField() string {
	return "state"
}

func (storage *PeopleStorageService) deleted() string {
	return "deleted"
}

func (storage *PeopleStorageService) personField() string {
	return "personId"
}
