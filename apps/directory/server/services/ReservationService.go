import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
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
	wanted, taken := addresses.Reservations{}.ForPerson(service.claimable(document), username, personId)
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
	return service.releaseUnless(failure, document, addresses.Reservations{}.WithoutDomain(service.reservation(document), parts.DomainId))
}

func (service *ReservationService) Pending(username string) (string, error) {
	document, failure := service.localParts.Find(username)
	request := cmp.Or(document.AddRequest, service.newRequest())
	pending := LocalPartDocument{Id: document.Id, LocalPart: username, DomainIds: []string{}, AddRequest: request}
	return request, service.markUnless(failure, document, pending)
}

func (service *ReservationService) ReleaseDomain(document LocalPartDocument, domainId string) error {
	return service.releaseUnless(nil, document, addresses.Reservations{}.WithoutDomain(service.reservation(document), domainId))
}

func (service *ReservationService) markUnless(failure error, document LocalPartDocument, pending LocalPartDocument) error {
	switch {
	case failure != nil:
		return failure
	case document.AddRequest == pending.AddRequest:
		return nil
	}
	return service.localParts.Store(pending)
}

func (service *ReservationService) claimable(document LocalPartDocument) addresses.Reservation {
	if document.Owner == "" {
		return addresses.Reservation{}
	}
	return service.reservation(document)
}

func (service *ReservationService) newRequest() string {
	random := make([]byte, service.requestBytes())
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (service *ReservationService) requestBytes() int {
	return 16
}

func (service *ReservationService) storeUnless(failure error, document LocalPartDocument, wanted addresses.Reservation) error {
	switch {
	case failure != nil:
		return failure
	case reflect.DeepEqual(service.reservation(document), service.normal(wanted)):
		return nil
	}
	return service.localParts.Store(service.stored(document, wanted))
}

func (service *ReservationService) releaseUnless(failure error, document LocalPartDocument, wanted addresses.Reservation) error {
	switch {
	case failure != nil:
		return failure
	case reflect.DeepEqual(service.reservation(document), service.normal(wanted)):
		return nil
	case len(wanted.DomainIds) > 0, wanted.Owner != addresses.Owners{}.Addresses():
		return service.localParts.Store(service.stored(document, wanted))
	}
	return service.localParts.Delete(document)
}

func (service *ReservationService) stored(document LocalPartDocument, wanted addresses.Reservation) LocalPartDocument {
	return LocalPartDocument{Id: document.Id, LocalPart: wanted.LocalPart, Owner: wanted.Owner, PersonId: wanted.PersonId, DomainIds: wanted.DomainIds}
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
