import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type L3MembershipReservationTest struct {
	behaviors *BehaviorContext
}

func (test *L3MembershipReservationTest) TestStoreGivesANewPersonAnId() {
	person := test.person()
	saved, err := test.people().Store(person)
	found, exists, findErr := test.people().ByPersonId(person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("<nil>", fmt.Sprint(findErr))
	Assert.IsTrue(saved.Id != "", "want a stored id")
	Assert.IsTrue(exists, "person not found")
	Assert.Equals(person.Username, found.Username)
	Assert.Equals("a.test", strings.Join(found.DomainIds, ","))
}

func (test *L3MembershipReservationTest) TestStoreOfAKnownPersonReplacesTheRecord() {
	saved, _ := test.people().Store(test.person())
	saved.DisplayName = "Changed Name"
	_, err := test.people().Store(saved)
	found, _, _ := test.people().ByPersonId(saved.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("Changed Name", found.DisplayName)
	Assert.Equals(saved.Id, found.Id)
}

func (test *L3MembershipReservationTest) TestByPersonIdOfAnUnknownPersonFindsNothing() {
	_, found, err := test.people().ByPersonId("kc-" + test.unique())
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(!found, "want nothing found")
}

func (test *L3MembershipReservationTest) TestAllListsAStoredPerson() {
	person := test.person()
	test.people().Store(person)
	people, err := test.people().All()
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(test.holds(people, person.PersonId), "person not listed")
}

func (test *L3MembershipReservationTest) TestFindOfAnUnknownLocalPartIsEmpty() {
	document, err := test.localParts().Find(test.username())
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("", document.Owner)
}

func (test *L3MembershipReservationTest) TestStoreOfALocalPartInsertsAndThenReplaces() {
	localPart := test.username()
	err := test.localParts().Store(LocalPartDocument{LocalPart: localPart, Owner: "person", PersonId: "kc-" + localPart})
	stored, _ := test.localParts().Find(localPart)
	stored.PersonId = "kc-changed"
	replaceErr := test.localParts().Store(stored)
	replaced, _ := test.localParts().Find(localPart)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("<nil>", fmt.Sprint(replaceErr))
	Assert.IsTrue(stored.Id != "", "want a stored id")
	Assert.Equals("kc-changed", replaced.PersonId)
	Assert.Equals(stored.Id, replaced.Id)
}

func (test *L3MembershipReservationTest) TestUsernameIsTrimmedAndLowered() {
	username, err := test.reservations().Username(" Ana.Example ")
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("ana.example", username)
}

func (test *L3MembershipReservationTest) TestUsernameWithASpaceIsInvalid() {
	_, err := test.reservations().Username("ana example")
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("username", invalid.Field)
}

func (test *L3MembershipReservationTest) TestUsernameThatIsARoleNameIsInvalid() {
	_, err := test.reservations().Username("abuse")
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("username", invalid.Field)
}

func (test *L3MembershipReservationTest) TestCurrentOfAFreeNameIsEmpty() {
	reservation, err := test.reservations().Current(test.username())
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("", reservation.Owner)
	Assert.Equals(0, len(reservation.DomainIds))
}

func (test *L3MembershipReservationTest) TestForPersonReservesTheNameForThatPerson() {
	username := test.username()
	err := test.reservations().ForPerson(username, "kc-"+username)
	again := test.reservations().ForPerson(username, "kc-"+username)
	reservation, _ := test.reservations().Current(username)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("<nil>", fmt.Sprint(again))
	Assert.Equals("person", reservation.Owner)
	Assert.Equals("kc-"+username, reservation.PersonId)
}

func (test *L3MembershipReservationTest) TestForPersonOfANameHeldByAnotherPersonIsADuplicate() {
	username := test.username()
	test.reservations().ForPerson(username, "kc-first")
	err := test.reservations().ForPerson(username, "kc-second")
	reservation, _ := test.reservations().Current(username)
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
	Assert.Equals("kc-first", reservation.PersonId)
}

func (test *L3MembershipReservationTest) TestForPersonOfANameHeldByAddressesIsADuplicate() {
	username := test.username()
	test.reservations().ForAddress(username + "@" + test.domain())
	err := test.reservations().ForPerson(username, "kc-"+username)
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *L3MembershipReservationTest) TestForPersonOfARoleNameIsInvalid() {
	err := test.reservations().ForPerson("hostmaster", "kc-hostmaster")
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("username", invalid.Field)
}

func (test *L3MembershipReservationTest) TestForAddressCollectsTheDomainsOfTheName() {
	localPart := test.username()
	first, second := test.domain(), test.domain()
	test.reservations().ForAddress(localPart + "@" + first)
	err := test.reservations().ForAddress(strings.ToUpper(localPart) + "@" + second)
	again := test.reservations().ForAddress(localPart + "@" + second)
	reservation, _ := test.reservations().Current(localPart)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("<nil>", fmt.Sprint(again))
	Assert.Equals("addresses", reservation.Owner)
	Assert.Equals(first+","+second, strings.Join(reservation.DomainIds, ","))
}

func (test *L3MembershipReservationTest) TestForAddressWithoutAnAtIsInvalid() {
	err := test.reservations().ForAddress("not-an-address")
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("address", invalid.Field)
}

func (test *L3MembershipReservationTest) TestForAddressOfAPersonsNameIsADuplicate() {
	username := test.username()
	test.reservations().ForPerson(username, "kc-"+username)
	err := test.reservations().ForAddress(username + "@" + test.domain())
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *L3MembershipReservationTest) TestReleaseDropsOneDomainOfTheName() {
	localPart := test.username()
	first, second := test.domain(), test.domain()
	test.reservations().ForAddress(localPart + "@" + first)
	test.reservations().ForAddress(localPart + "@" + second)
	err := test.reservations().Release(localPart + "@" + first)
	reservation, _ := test.reservations().Current(localPart)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(second, strings.Join(reservation.DomainIds, ","))
}

func (test *L3MembershipReservationTest) TestReleaseKeepsTheNameOfAPerson() {
	username := test.username()
	test.reservations().ForPerson(username, "kc-"+username)
	err := test.reservations().Release(username + "@" + test.domain())
	reservation, _ := test.reservations().Current(username)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("person", reservation.Owner)
	Assert.Equals("kc-"+username, reservation.PersonId)
}

func (test *L3MembershipReservationTest) TestReleaseOfAnInvalidAddressDoesNothing() {
	err := test.reservations().Release("not-an-address")
	Assert.Equals("<nil>", fmt.Sprint(err))
}

func (test *L3MembershipReservationTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *L3MembershipReservationTest) username() string {
	return "r" + test.unique()
}

func (test *L3MembershipReservationTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3MembershipReservationTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipReservationTest) localParts() *LocalPartStorageService {
	return &LocalPartStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipReservationTest) reservations() *ReservationService {
	return &ReservationService{localParts: test.localParts()}
}

func (test *L3MembershipReservationTest) domain() string {
	return "r" + test.unique() + ".test"
}

func (test *L3MembershipReservationTest) person() PersonDocument {
	username := test.username()
	return PersonDocument{PersonId: "kc-" + username, Username: username, DisplayName: "Person " + username, DomainIds: []string{"a.test"}, State: "active"}
}

func (test *L3MembershipReservationTest) holds(people []PersonDocument, personId string) bool {
	for _, person := range people {
		if person.PersonId == personId {
			return true
		}
	}
	return false
}
