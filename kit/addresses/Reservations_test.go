package addresses

import (
	"errors"
	"reflect"
	"testing"
)

func assertReservation(t *testing.T, got Reservation, want Reservation) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func assertTaken(t *testing.T, got Reservation, err error, localPart string) {
	t.Helper()
	var taken Taken
	if !errors.As(err, &taken) {
		t.Fatalf("expected Taken, got %v", err)
	}
	assertText(t, taken.LocalPart, localPart)
	assertReservation(t, got, Reservation{})
}

func personReservation() Reservation {
	return Reservation{LocalPart: "ana", Owner: Owners{}.Person(), PersonId: "person-1"}
}

func addressesReservation() Reservation {
	return Reservation{LocalPart: "sales", Owner: Owners{}.Addresses(), DomainIds: []string{"a.example", "b.example"}}
}

func TestForPersonCreatesNewReservation(t *testing.T) {
	got, err := Reservations{}.ForPerson(Reservation{}, " Ana ", "person-1")
	assertAccepted(t, err)
	assertReservation(t, got, personReservation())
}

func TestForPersonRetryReturnsCurrent(t *testing.T) {
	got, err := Reservations{}.ForPerson(personReservation(), "ana", "person-1")
	assertAccepted(t, err)
	assertReservation(t, got, personReservation())
}

func TestForPersonRefusesOtherPerson(t *testing.T) {
	got, err := Reservations{}.ForPerson(personReservation(), "ana", "person-2")
	assertTaken(t, got, err, "ana")
}

func TestForPersonRefusesLocalPartOfAddresses(t *testing.T) {
	got, err := Reservations{}.ForPerson(addressesReservation(), "Sales", "person-1")
	assertTaken(t, got, err, "sales")
}

func TestForPersonRefusesRoleName(t *testing.T) {
	got, err := Reservations{}.ForPerson(Reservation{}, "abuse", "person-1")
	assertRefusedAs(t, err, NameKinds{}.Username())
	assertReservation(t, got, Reservation{})
}

func TestForAddressesCreatesNewReservation(t *testing.T) {
	got, err := Reservations{}.ForAddresses(Reservation{}, "Postmaster", "A.Example")
	assertAccepted(t, err)
	assertReservation(t, got, Reservation{LocalPart: "postmaster", Owner: Owners{}.Addresses(), DomainIds: []string{"a.example"}})
}

func TestForAddressesAppendsNewDomain(t *testing.T) {
	current := addressesReservation()
	got, err := Reservations{}.ForAddresses(current, "sales", "c.example")
	assertAccepted(t, err)
	assertReservation(t, got, Reservation{LocalPart: "sales", Owner: Owners{}.Addresses(), DomainIds: []string{"a.example", "b.example", "c.example"}})
	assertReservation(t, current, addressesReservation())
}

func TestForAddressesKeepsKnownDomain(t *testing.T) {
	got, err := Reservations{}.ForAddresses(addressesReservation(), "sales", "B.EXAMPLE")
	assertAccepted(t, err)
	assertReservation(t, got, addressesReservation())
}

func TestForAddressesRefusesPersonPrefix(t *testing.T) {
	got, err := Reservations{}.ForAddresses(personReservation(), "ana", "a.example")
	assertTaken(t, got, err, "ana")
}

func TestForAddressesRefusesBadLocalPart(t *testing.T) {
	got, err := Reservations{}.ForAddresses(Reservation{}, "sales+eu", "a.example")
	assertRefusedAs(t, err, NameKinds{}.LocalPart())
	assertReservation(t, got, Reservation{})
}

func TestForAddressesRefusesBadDomain(t *testing.T) {
	got, err := Reservations{}.ForAddresses(Reservation{}, "sales", "*.example")
	assertRefusedAs(t, err, NameKinds{}.Domain())
	assertReservation(t, got, Reservation{})
}

func TestWithoutDomainRemovesDomainOfAddresses(t *testing.T) {
	current := addressesReservation()
	got := Reservations{}.WithoutDomain(current, " A.Example ")
	assertReservation(t, got, Reservation{LocalPart: "sales", Owner: Owners{}.Addresses(), DomainIds: []string{"b.example"}})
	assertReservation(t, current, addressesReservation())
}

func TestWithoutDomainKeepsUnknownDomain(t *testing.T) {
	got := Reservations{}.WithoutDomain(addressesReservation(), "c.example")
	assertReservation(t, got, addressesReservation())
}

func TestWithoutDomainKeepsPersonReservation(t *testing.T) {
	got := Reservations{}.WithoutDomain(personReservation(), "a.example")
	assertReservation(t, got, personReservation())
}
