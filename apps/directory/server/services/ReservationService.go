import (
	"cmp"
	"errors"
	"reflect"

	"mesail.dev/packages/addresses"
)

// @Service
type ReservationService struct {
	// @Needs
	localParts *LocalPartStorageService
}

func (service *ReservationService) Username(raw string) (string, error) {
	username, invalid := addresses.Username{}.Normalized(raw)
	return username, service.translated(invalid, service.usernameField())
}

func (service *ReservationService) Current(localPart string) (addresses.Reservation, error) {
	document, failure := service.localParts.Find(localPart)
	return service.reservation(document), failure
}

func (service *ReservationService) ForPerson(username string, personId string) error {
	document, failure := service.localParts.Find(username)
	wanted, taken := addresses.Reservations{}.ForPerson(service.reservation(document), username, personId)
	return service.storeUnless(cmp.Or(failure, service.translated(taken, service.usernameField())), document, wanted)
}

func (service *ReservationService) ForAddress(address string) error {
	parts, invalid := addresses.Address{}.Parsed(address)
	document, failure := service.localParts.Find(parts.LocalPart)
	wanted, taken := addresses.Reservations{}.ForAddresses(service.reservation(document), parts.LocalPart, parts.DomainId)
	refusal := cmp.Or(service.translated(invalid, service.addressField()), failure, service.translated(taken, service.addressField()))
	return service.storeUnless(refusal, document, wanted)
}

func (service *ReservationService) Release(address string) error {
	parts, _ := addresses.Address{}.Parsed(address)
	document, failure := service.localParts.Find(parts.LocalPart)
	return service.storeUnless(failure, document, addresses.Reservations{}.WithoutDomain(service.reservation(document), parts.DomainId))
}

func (service *ReservationService) storeUnless(failure error, document LocalPartDocument, wanted addresses.Reservation) error {
	switch {
	case failure != nil:
		return failure
	case reflect.DeepEqual(service.reservation(document), service.normal(wanted)):
		return nil
	}
	return service.localParts.Store(LocalPartDocument{Id: document.Id, LocalPart: wanted.LocalPart, Owner: wanted.Owner, PersonId: wanted.PersonId, DomainIds: wanted.DomainIds})
}

func (service *ReservationService) reservation(document LocalPartDocument) addresses.Reservation {
	return service.normal(addresses.Reservation{LocalPart: document.LocalPart, Owner: document.Owner, PersonId: document.PersonId, DomainIds: document.DomainIds})
}

func (service *ReservationService) normal(reservation addresses.Reservation) addresses.Reservation {
	reservation.DomainIds = append([]string{}, reservation.DomainIds...)
	return reservation
}

func (service *ReservationService) translated(failure error, field string) error {
	switch {
	case failure == nil:
		return nil
	case errors.As(failure, &addresses.Taken{}):
		return &Duplicate{Message: failure.Error()}
	}
	return &InvalidRequest{Field: field, Message: failure.Error()}
}

func (service *ReservationService) usernameField() string {
	return "username"
}

func (service *ReservationService) addressField() string {
	return "address"
}
