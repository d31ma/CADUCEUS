import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type L3MembershipWriterTest struct {
	behaviors *BehaviorContext
}

func (test *L3MembershipWriterTest) TestJoinStoresTheMemberAndItsPrimaryAddress() {
	domain := test.world()
	defer test.dropDomain(domain)
	person := test.person("active")
	saved, err := test.writer().Join(person, domain, true)
	route, found, _ := test.addresses().Find(person.Username + "@" + domain)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(saved.Id != "", "want a stored id")
	Assert.Equals(person.PersonId, saved.MemberId)
	Assert.Equals(person.Username+"@"+domain, saved.PrimaryAddress)
	Assert.IsTrue(saved.Admin, "want an admin")
	Assert.Equals("active", saved.State)
	Assert.IsTrue(found, "primary address not stored")
	Assert.Equals("member", route.Kind)
	Assert.Equals(person.PersonId, route.TargetId)
}

func (test *L3MembershipWriterTest) TestJoinOfADisabledPersonStoresADisabledMember() {
	domain := test.world()
	defer test.dropDomain(domain)
	saved, err := test.writer().Join(test.person("disabled"), domain, false)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("disabled", saved.State)
}

func (test *L3MembershipWriterTest) TestJoinAgainUpdatesTheSameMember() {
	domain := test.world()
	defer test.dropDomain(domain)
	person := test.person("active")
	test.writer().Join(person, domain, false)
	_, err := test.writer().Join(person, domain, true)
	members, _ := test.members().All(domain)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(1, len(members))
	Assert.IsTrue(members[0].Admin, "want an admin")
}

func (test *L3MembershipWriterTest) TestUpdateSavesTheChangedMember() {
	domain := test.world()
	defer test.dropDomain(domain)
	person := test.person("active")
	saved, _ := test.writer().Join(person, domain, false)
	saved.DisplayName = "Renamed Person"
	_, err := test.writer().Update(saved)
	stored, _, _ := test.members().Find(domain, person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("Renamed Person", stored.DisplayName)
}

func (test *L3MembershipWriterTest) TestLeaveMarksTheMemberRemovedAndDeletesItsAddress() {
	domain := test.world()
	defer test.dropDomain(domain)
	person := test.person("active")
	saved, _ := test.writer().Join(person, domain, true)
	err := test.writer().Leave(saved)
	stored, _, _ := test.members().Find(domain, person.PersonId)
	_, found, _ := test.addresses().Find(saved.PrimaryAddress)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("removed", stored.State)
	Assert.IsTrue(!stored.Admin, "want no admin")
	Assert.IsTrue(!found, "primary address still stored")
}

func (test *L3MembershipWriterTest) TestLeaveKeepsAnAddressThatPointsAtSomethingElse() {
	domain := test.world()
	defer test.dropDomain(domain)
	person := test.person("active")
	address := person.Username + "@" + domain
	test.addresses().Insert(AddressDocument{Address: address, DomainId: domain, Kind: "alias", TargetId: "kc-other"})
	saved, _ := test.writer().Join(person, domain, false)
	err := test.writer().Leave(saved)
	route, found, _ := test.addresses().Find(address)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(found, "alias address deleted")
	Assert.Equals("alias", route.Kind)
}

func (test *L3MembershipWriterTest) TestLeaveDomainRemovesAnActiveMember() {
	domain := test.world()
	defer test.dropDomain(domain)
	person := test.person("active")
	test.writer().Join(person, domain, false)
	err := test.writer().LeaveDomain(person.PersonId, domain)
	stored, _, _ := test.members().Find(domain, person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("removed", stored.State)
}

func (test *L3MembershipWriterTest) TestLeaveDomainOfAnUnknownMemberDoesNothing() {
	domain := test.world()
	defer test.dropDomain(domain)
	err := test.writer().LeaveDomain("kc-"+test.unique(), domain)
	members, _ := test.members().All(domain)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(0, len(members))
}

func (test *L3MembershipWriterTest) TestLeaveDomainOfARemovedMemberDoesNothing() {
	domain := test.world()
	defer test.dropDomain(domain)
	person := test.person("active")
	saved, _ := test.writer().Join(person, domain, false)
	test.writer().Leave(saved)
	err := test.writer().LeaveDomain(person.PersonId, domain)
	stored, _, _ := test.members().Find(domain, person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("removed", stored.State)
}

func (test *L3MembershipWriterTest) TestMirrorCopiesTheNameAndStateOfThePerson() {
	domain := test.world()
	defer test.dropDomain(domain)
	person := test.person("active")
	test.writer().Join(person, domain, false)
	person.DisplayName = "Mirrored Name"
	person.State = "disabled"
	err := test.writer().MirrorAll(PersonRules{}.With(person, domain))
	stored, _, _ := test.members().Find(domain, person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("Mirrored Name", stored.DisplayName)
	Assert.Equals("disabled", stored.State)
}

func (test *L3MembershipWriterTest) TestMirrorKeepsARemovedMemberRemoved() {
	domain := test.world()
	defer test.dropDomain(domain)
	person := test.person("active")
	saved, _ := test.writer().Join(person, domain, false)
	test.writer().Leave(saved)
	person.DisplayName = "Mirrored Name"
	err := test.writer().MirrorAll(PersonRules{}.With(person, domain))
	stored, _, _ := test.members().Find(domain, person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("Mirrored Name", stored.DisplayName)
	Assert.Equals("removed", stored.State)
}

func (test *L3MembershipWriterTest) TestMirrorOfAPersonWhoIsNotAMemberDoesNothing() {
	domain := test.world()
	defer test.dropDomain(domain)
	person := test.person("active")
	err := test.writer().MirrorAll(PersonRules{}.With(person, domain))
	_, found, _ := test.members().Find(domain, person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(!found, "member stored by a mirror")
}

func (test *L3MembershipWriterTest) TestActiveReturnsAnActiveMember() {
	domain := test.world()
	defer test.dropDomain(domain)
	person := test.person("active")
	test.writer().Join(person, domain, false)
	member, err := test.writer().Active(domain, person.PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(person.PersonId, member.MemberId)
}

func (test *L3MembershipWriterTest) TestActiveOfAnUnknownMemberIsNotFound() {
	domain := test.world()
	defer test.dropDomain(domain)
	_, err := test.writer().Active(domain, "kc-"+test.unique())
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *L3MembershipWriterTest) TestActiveOfARemovedMemberIsNotFound() {
	domain := test.world()
	defer test.dropDomain(domain)
	person := test.person("active")
	saved, _ := test.writer().Join(person, domain, false)
	test.writer().Leave(saved)
	_, err := test.writer().Active(domain, person.PersonId)
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *L3MembershipWriterTest) TestOtherAdminRefusesTheOnlyAdmin() {
	domain := test.world()
	defer test.dropDomain(domain)
	saved, _ := test.writer().Join(test.person("active"), domain, true)
	err := test.writer().OtherAdmin(saved)
	var lastAdmin *LastAdmin
	Assert.IsTrue(errors.As(err, &lastAdmin), "want LastAdmin")
}

func (test *L3MembershipWriterTest) TestOtherAdminAcceptsAnotherActiveAdmin() {
	domain := test.world()
	defer test.dropDomain(domain)
	saved, _ := test.writer().Join(test.person("active"), domain, true)
	test.writer().Join(test.person("active"), domain, true)
	err := test.writer().OtherAdmin(saved)
	Assert.Equals("<nil>", fmt.Sprint(err))
}

func (test *L3MembershipWriterTest) TestOtherAdminIgnoresADisabledAdmin() {
	domain := test.world()
	defer test.dropDomain(domain)
	saved, _ := test.writer().Join(test.person("active"), domain, true)
	test.writer().Join(test.person("disabled"), domain, true)
	err := test.writer().OtherAdmin(saved)
	var lastAdmin *LastAdmin
	Assert.IsTrue(errors.As(err, &lastAdmin), "want LastAdmin")
}

func (test *L3MembershipWriterTest) world() string {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{}}`)
	domainId := "w3-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
	return domainId
}

func (test *L3MembershipWriterTest) dropDomain(domainId string) {
	domain, _, _ := test.domains().Find(domainId)
	test.domains().Delete(domain)
	(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId)
}

func (test *L3MembershipWriterTest) person(state string) PersonDocument {
	username := "w" + test.unique()
	return PersonDocument{PersonId: "kc-" + username, Username: username, DisplayName: "Person " + username, State: state}
}

func (test *L3MembershipWriterTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *L3MembershipWriterTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *L3MembershipWriterTest) members() *MemberStorageService {
	return &MemberStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipWriterTest) addresses() *AddressStorageService {
	return &AddressStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipWriterTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *L3MembershipWriterTest) writer() *MemberWriterService {
	names := &CollectionNamesService{}
	store := &OutboxStoreService{domains: test.domains(), members: test.members(), names: names}
	relay := &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
	return &MemberWriterService{members: test.members(), addresses: test.addresses(), events: &EventService{settings: &SettingsService{}}, relay: relay}
}
