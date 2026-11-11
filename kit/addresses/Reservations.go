package addresses

import (
	"slices"
	"strings"
)

type Reservations struct{}

func (reservations Reservations) ForPerson(current Reservation, localPart string, personId string) (Reservation, error) {
	value, err := Username{}.Normalized(localPart)
	if err != nil {
		return Reservation{}, err
	}
	wanted := Reservation{LocalPart: value, Owner: Owners{}.Person(), PersonId: personId}
	existing := reservations.existingOr(current, wanted)
	err = reservations.checkSameOwner(existing, wanted)
	if err != nil {
		return Reservation{}, err
	}
	return existing, nil
}

func (reservations Reservations) ForAddresses(current Reservation, localPart string, domainId string) (Reservation, error) {
	value, err := LocalPart{}.Normalized(localPart)
	if err != nil {
		return Reservation{}, err
	}
	domain, err := DomainName{}.Normalized(domainId)
	if err != nil {
		return Reservation{}, err
	}
	wanted := Reservation{LocalPart: value, Owner: Owners{}.Addresses()}
	existing := reservations.existingOr(current, wanted)
	err = reservations.checkSameOwner(existing, wanted)
	if err != nil {
		return Reservation{}, err
	}
	return reservations.withDomain(existing, domain), nil
}

func (reservations Reservations) WithoutDomain(current Reservation, domainId string) Reservation {
	return reservations.withoutDomainForAddresses(current, strings.ToLower(strings.TrimSpace(domainId)))
}

func (Reservations) existingOr(current Reservation, wanted Reservation) Reservation {
	if current.LocalPart == "" {
		return wanted
	}
	return current
}

func (Reservations) checkSameOwner(existing Reservation, wanted Reservation) error {
	if existing.Owner != wanted.Owner || existing.PersonId != wanted.PersonId {
		return Taken{LocalPart: wanted.LocalPart}
	}
	return nil
}

func (Reservations) withDomain(existing Reservation, domainId string) Reservation {
	if slices.Contains(existing.DomainIds, domainId) {
		return existing
	}
	changed := existing
	changed.DomainIds = append(slices.Clone(existing.DomainIds), domainId)
	return changed
}

func (reservations Reservations) withoutDomainForAddresses(current Reservation, domainId string) Reservation {
	if current.Owner != (Owners{}).Addresses() {
		return current
	}
	changed := current
	changed.DomainIds = reservations.domainsWithout(current.DomainIds, domainId)
	return changed
}

func (Reservations) domainsWithout(domainIds []string, domainId string) []string {
	return slices.DeleteFunc(slices.Clone(domainIds), func(candidate string) bool { return candidate == domainId })
}
