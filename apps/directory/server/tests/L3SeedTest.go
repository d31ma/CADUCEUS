import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type L3SeedTest struct {
	behaviors *BehaviorContext
}

func (test *L3SeedTest) TestSeedWritesThePersonTheMembersAndTheReservation() {
	domain := test.world(true)
	defer test.dropDomain(domain)
	seed := test.seed(domain)
	record, err := test.service().Seed(seed)
	member, found, _ := test.members().Find(domain, seed.PersonId)
	reservation, _ := (&ReservationService{localParts: test.localParts()}).Current(seed.Username)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(seed.PersonId, record.PersonId)
	Assert.Equals(domain, record.DefaultDomainId)
	Assert.Equals(1, len(record.AdminDomainIds))
	Assert.IsTrue(found, "member not written")
	Assert.Equals(seed.Username+"@"+domain, member.PrimaryAddress)
	Assert.IsTrue(member.Admin, "want an admin member")
	Assert.Equals(seed.PersonId, reservation.PersonId)
}

func (test *L3SeedTest) TestSeedingTheSamePersonTwiceKeepsOnePerson() {
	domain := test.world(true)
	defer test.dropDomain(domain)
	seed := test.seed(domain)
	test.service().Seed(seed)
	record, err := test.service().Seed(seed)
	people, _ := test.people().All()
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(1, len(record.DomainIds))
	Assert.Equals(1, test.count(people, seed.PersonId))
}

func (test *L3SeedTest) TestSeedRefusesMoreThanTwoDomainsInOneCall() {
	domain := test.world(true)
	defer test.dropDomain(domain)
	seed := test.seed(domain)
	seed.Memberships = append(seed.Memberships, SeedMembership{DomainId: domain}, SeedMembership{DomainId: domain})
	_, err := test.service().Seed(seed)
	_, found, _ := test.members().Find(domain, seed.PersonId)
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("memberships", invalid.Field)
	Assert.IsTrue(!found, "no member may be written")
}

func (test *L3SeedTest) TestSeedIsNotFoundUnlessTheSettingIsOn() {
	domain := test.world(false)
	defer test.dropDomain(domain)
	_, err := test.service().Seed(test.seed(domain))
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *L3SeedTest) TestSeedRefusesADomainThatIsNotHosted() {
	domain := test.world(true)
	defer test.dropDomain(domain)
	_, err := test.service().Seed(test.seed("unknown-" + test.unique() + ".test"))
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
}

func (test *L3SeedTest) TestSeedRefusesAPrefixOfAnotherPerson() {
	domain := test.world(true)
	defer test.dropDomain(domain)
	seed := test.seed(domain)
	test.service().Seed(seed)
	other := seed
	other.PersonId = "kc-other-" + test.unique()
	_, err := test.service().Seed(other)
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *L3SeedTest) TestSeedNeedsAPersonIdAndADisplayName() {
	domain := test.world(true)
	defer test.dropDomain(domain)
	seed := test.seed(domain)
	seed.DisplayName = " "
	_, err := test.service().Seed(seed)
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("displayName", invalid.Field)
}

func (test *L3SeedTest) TestSeedWithoutAPersonIdIsRefused() {
	domain := test.world(true)
	defer test.dropDomain(domain)
	seed := test.seed(domain)
	seed.PersonId = ""
	_, err := test.service().Seed(seed)
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("personId", invalid.Field)
}

func (test *L3SeedTest) TestTheSeedRouteRefusesARequestWithoutAValidToken() {
	controller := &PeopleController{callers: &CallerService{}, seeds: test.service()}
	_, err := controller.PostSeed("Bearer forged", PersonSeed{})
	var refused *Unauthenticated
	Assert.IsTrue(errors.As(err, &refused), "want Unauthenticated")
}

func (test *L3SeedTest) count(people []PersonDocument, personId string) int {
	found := 0
	for _, person := range people {
		found += map[bool]int{true: 1, false: 0}[person.PersonId == personId]
	}
	return found
}

func (test *L3SeedTest) seed(domain string) PersonSeed {
	username := "s" + test.unique()
	return PersonSeed{PersonId: "kc-" + username, Username: username, DisplayName: "Seed " + username, Memberships: []SeedMembership{{DomainId: domain, Admin: true}}}
}

func (test *L3SeedTest) world(enabled bool) string {
	os.Setenv("YON_APPLICATION_JSON", fmt.Sprintf(`{"yon":{"directory":{"developmentseed":"%t"}}}`, enabled))
	domainId := "s3-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
	return domainId
}

func (test *L3SeedTest) dropDomain(domainId string) {
	domain, _, _ := test.domains().Find(domainId)
	test.domains().Delete(domain)
	(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
}

func (test *L3SeedTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *L3SeedTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3SeedTest) members() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3SeedTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3SeedTest) localParts() *LocalPartStorageService {
	return &LocalPartStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3SeedTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3SeedTest) service() *PersonSeedService {
	names := &CollectionNamesService{}
	store := &OutboxStoreService{domains: test.domains(), members: test.members(), names: names}
	relay := &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
	writer := &MemberWriterService{members: test.members(), addresses: &AddressStorageService{documents: test.documents(), names: names}, events: &EventService{settings: &SettingsService{}}, relay: relay}
	return &PersonSeedService{settings: &SettingsService{}, people: test.people(), reservations: &ReservationService{localParts: test.localParts()}, tenants: &TenantService{domains: test.domains()}, writer: writer}
}
