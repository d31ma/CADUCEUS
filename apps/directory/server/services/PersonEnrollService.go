import (
	"cmp"
	"strings"

	"mesail.dev/packages/addresses"
)

// @Service
type PersonEnrollService struct {
	// @Needs
	people *PeopleStorageService
	// @Needs
	reservations *ReservationService
	// @Needs
	accounts *AccountAdminService
}

func (service *PersonEnrollService) Enroll(request AddMemberRequest) (PersonDocument, bool, error) {
	username, invalid := service.reservations.Username(request.Username)
	reservation, failure := service.currentUnless(invalid, username)
	request.Username = username
	return service.byOwner(failure, reservation, request)
}

func (service *PersonEnrollService) currentUnless(invalid error, username string) (addresses.Reservation, error) {
	if invalid != nil {
		return addresses.Reservation{}, invalid
	}
	return service.reservations.Current(username)
}

func (service *PersonEnrollService) byOwner(failure error, reservation addresses.Reservation, request AddMemberRequest) (PersonDocument, bool, error) {
	switch {
	case failure != nil:
		return PersonDocument{}, false, failure
	case reservation.Owner == addresses.Owners{}.Addresses():
		return PersonDocument{}, false, &Duplicate{Message: request.Username + " is used by an alias or a channel address"}
	case reservation.Owner == addresses.Owners{}.Person():
		return service.reserved(reservation.PersonId, request)
	}
	return service.created(request)
}

func (service *PersonEnrollService) reserved(personId string, request AddMemberRequest) (PersonDocument, bool, error) {
	person, found, failure := service.people.ByPersonId(personId)
	fresh, invalid := service.fresh(personId, request)
	return service.chosen(found, person, fresh), found, cmp.Or(failure, service.chosenRefusal(found, service.usable(person), invalid))
}

func (service *PersonEnrollService) created(request AddMemberRequest) (PersonDocument, bool, error) {
	_, invalid := service.fresh("", request)
	personId, failure := service.createUnless(cmp.Or(invalid, service.passwordGiven(request.FirstPassword)), request)
	failure = service.reserveUnless(failure, request.Username, personId)
	person, _ := service.fresh(personId, request)
	return person, false, failure
}

func (service *PersonEnrollService) createUnless(failure error, request AddMemberRequest) (string, error) {
	if failure != nil {
		return "", failure
	}
	return service.accounts.Create(request.Username, request.FirstPassword)
}

func (service *PersonEnrollService) reserveUnless(failure error, username string, personId string) error {
	if failure != nil {
		return failure
	}
	return service.reservations.ForPerson(username, personId)
}

func (service *PersonEnrollService) chosen(found bool, person PersonDocument, fresh PersonDocument) PersonDocument {
	if found {
		return person
	}
	return fresh
}

func (service *PersonEnrollService) chosenRefusal(found bool, unusable error, invalid error) error {
	if found {
		return unusable
	}
	return invalid
}

func (service *PersonEnrollService) fresh(personId string, request AddMemberRequest) (PersonDocument, error) {
	displayName := strings.TrimSpace(request.DisplayName)
	person := PersonDocument{PersonId: personId, Username: request.Username, DisplayName: displayName, DomainIds: []string{}, AdminDomainIds: []string{}, State: service.active()}
	return person, service.named(displayName)
}

func (service *PersonEnrollService) named(displayName string) error {
	if displayName == "" {
		return &InvalidRequest{Field: "displayName", Message: "a display name is required for a new person"}
	}
	return nil
}

func (service *PersonEnrollService) passwordGiven(password string) error {
	if strings.TrimSpace(password) == "" {
		return &InvalidRequest{Field: "firstPassword", Message: "a first password is required for a new person"}
	}
	return nil
}

func (service *PersonEnrollService) usable(person PersonDocument) error {
	if person.State == service.deleted() {
		return &Duplicate{Message: person.Username + " belonged to a deleted person and is never reused"}
	}
	return nil
}

func (service *PersonEnrollService) active() string {
	return "active"
}

func (service *PersonEnrollService) deleted() string {
	return "deleted"
}
