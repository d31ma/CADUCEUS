import (
	"cmp"
	"errors"
	"slices"
	"strings"
)

// @Service
type PersonSeedService struct {
	// @Needs
	settings *SettingsService
	// @Needs
	people *PeopleStorageService
	// @Needs
	reservations *ReservationService
	// @Needs
	tenants *TenantService
	// @Needs
	writer *MemberWriterService
}

func (service *PersonSeedService) Seed(seed PersonSeed) (PersonRecord, error) {
	username, invalid := service.reservations.Username(seed.Username)
	failure := cmp.Or(service.enabled(), invalid, service.complete(seed), service.hosted(seed.Memberships))
	failure = Steps{}.Unless(failure, func() error { return service.reservations.ForPerson(username, seed.PersonId) })
	existing, _, readFailure := service.people.ByPersonId(seed.PersonId)
	stored, failure := service.storeUnless(cmp.Or(failure, readFailure), service.seeded(existing, seed, username))
	failure = Steps{}.Unless(failure, func() error { return service.joinAll(stored, seed.Memberships) })
	return PersonRecord{PersonId: stored.PersonId, Username: stored.Username, DisplayName: stored.DisplayName, DomainIds: stored.DomainIds, AdminDomainIds: stored.AdminDomainIds, DefaultDomainId: stored.DefaultDomainId, State: stored.State}, failure
}

func (service *PersonSeedService) enabled() error {
	if service.settings.Settings().DevelopmentSeed != "true" {
		return &NotFound{Message: "not found"}
	}
	return nil
}

func (service *PersonSeedService) complete(seed PersonSeed) error {
	switch {
	case strings.TrimSpace(seed.PersonId) == "":
		return &InvalidRequest{Field: "personId", Message: "a person id is required"}
	case strings.TrimSpace(seed.DisplayName) == "":
		return &InvalidRequest{Field: "displayName", Message: "a display name is required"}
	}
	return nil
}

func (service *PersonSeedService) hosted(memberships []SeedMembership) error {
	failures := []error{}
	for _, membership := range memberships {
		_, failure := service.tenants.Require(strings.ToLower(membership.DomainId))
		failures = append(failures, failure)
	}
	return errors.Join(failures...)
}

func (service *PersonSeedService) seeded(existing PersonDocument, seed PersonSeed, username string) PersonDocument {
	person := existing
	person.PersonId = seed.PersonId
	person.Username = username
	person.DisplayName = strings.TrimSpace(seed.DisplayName)
	person.State = cmp.Or(existing.State, service.active())
	for _, membership := range seed.Memberships {
		domainId := strings.ToLower(membership.DomainId)
		person = PersonRules{}.WithAdmin(PersonRules{}.With(person, domainId), domainId, membership.Admin)
	}
	return person
}

func (service *PersonSeedService) storeUnless(failure error, person PersonDocument) (PersonDocument, error) {
	if failure != nil {
		return person, failure
	}
	return service.people.Store(person)
}

func (service *PersonSeedService) joinAll(person PersonDocument, memberships []SeedMembership) error {
	failures := []error{}
	for _, membership := range memberships {
		domainId := strings.ToLower(membership.DomainId)
		_, failure := service.writer.Join(person, domainId, slices.Contains(person.AdminDomainIds, domainId))
		failures = append(failures, failure)
	}
	return errors.Join(failures...)
}

func (service *PersonSeedService) active() string {
	return "active"
}
