import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"mesail.dev/packages/oidc"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type MemberServiceTest struct {
	behaviors *BehaviorContext
}

func (test *MemberServiceTest) TestSearchFindsMembersByName() {
	service, _, domain := test.world()
	test.join(service, domain, "bo-id", "bo", "Bo Member")
	test.join(service, domain, "ana-id", "ana", "Ana Admin")
	page, err := test.lister().Search(Admission{Caller: test.caller(domain, "Bo Member")}, MemberQuery{Text: "ana"}, nil)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(1, len(page.Items))
	Assert.Equals("ana-id", page.Items[0].MemberId)
	Assert.Equals("", page.NextCursor)
}

func (test *MemberServiceTest) TestSearchNeverReturnsAnotherDomain() {
	service, _, domain := test.world()
	otherDomain := test.hosted("member-other.test", "active")
	test.join(service, otherDomain, "eve-id", "eve", "Eve")
	page, err := test.lister().Search(Admission{Caller: test.caller(domain, "Bo Member")}, MemberQuery{Text: "eve"}, nil)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(0, len(page.Items))
}

func (test *MemberServiceTest) TestSearchRefusesALimitAboveOneHundred() {
	_, _, domain := test.world()
	_, err := test.lister().Search(Admission{Caller: test.caller(domain, "Bo")}, MemberQuery{Limit: 101}, nil)
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("limit", invalid.Field)
}

func (test *MemberServiceTest) TestSearchPagesWithACursor() {
	service, _, domain := test.world()
	test.join(service, domain, "bo-id", "bo", "Bo Member")
	test.join(service, domain, "ana-id", "ana", "Ana Admin")
	first, _ := test.lister().Search(Admission{Caller: test.caller(domain, "Bo Member")}, MemberQuery{Limit: 1}, nil)
	second, err := test.lister().Search(Admission{Caller: test.caller(domain, "Bo Member")}, MemberQuery{Limit: 1, Cursor: first.NextCursor}, nil)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("1", first.NextCursor)
	Assert.Equals(1, len(second.Items))
	Assert.Equals("", second.NextCursor)
}

func (test *MemberServiceTest) TestSearchRefusesACursorItDidNotGiveOut() {
	_, _, domain := test.world()
	_, err := test.lister().Search(Admission{Caller: test.caller(domain, "Bo")}, MemberQuery{Cursor: "abc"}, nil)
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("cursor", invalid.Field)
}

func (test *MemberServiceTest) TestSummaryOfAMemberOfTheCallersDomain() {
	service, _, domain := test.world()
	test.join(service, domain, "ana-id", "ana", "Ana Admin")
	summary, err := test.lister().Summary(Admission{Caller: test.caller(domain, "Bo")}, "ana-id")
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("Ana Admin", summary.DisplayName)
	Assert.Equals("ana@"+domain, summary.PrimaryAddress)
}

func (test *MemberServiceTest) TestSummaryOfAMemberOfAnotherDomainIsNotFound() {
	service, _, domain := test.world()
	otherDomain := test.hosted("member-other.test", "active")
	test.join(service, otherDomain, "eve-id", "eve", "Eve")
	_, err := test.lister().Summary(Admission{Caller: test.caller(domain, "Bo")}, "eve-id")
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *MemberServiceTest) TestSummaryForARefusedCallerIsTheRefusal() {
	_, _, domain := test.world()
	_, err := test.lister().Summary(Admission{Caller: test.caller(domain, "Bo"), Refusal: &Unauthenticated{}}, "bo-id")
	var refused *Unauthenticated
	Assert.IsTrue(errors.As(err, &refused), "want Unauthenticated")
}

func (test *MemberServiceTest) caller(domain string, name string) oidc.Caller {
	return oidc.Caller{DomainId: domain, MemberId: "bo-id", DisplayName: name, Email: "Bo@" + strings.ToUpper(domain)}
}

func (test *MemberServiceTest) world() (*MemberWriterService, *MemberStorageService, string) {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{}}`)
	documents := &DocumentsService{dataSource: DataSource{}}
	members := &MemberStorageService{documents: documents, names: &CollectionNamesService{}}
	addresses := &AddressStorageService{documents: documents, names: &CollectionNamesService{}}
	return &MemberWriterService{members: members, addresses: addresses, events: &EventService{settings: &SettingsService{}}, relay: test.relay(documents)}, members, test.hosted("member-unit.test", "active")
}

func (test *MemberServiceTest) join(writer *MemberWriterService, domainId string, personId string, username string, name string) {
	_, err := writer.Join(PersonDocument{PersonId: personId, Username: username, DisplayName: name, State: "active"}, domainId, false)
	Assert.Equals("<nil>", fmt.Sprint(err))
}

func (test *MemberServiceTest) hosted(name string, state string) string {
	domainId := test.reset(name)
	documents := &DocumentsService{dataSource: DataSource{}}
	collections := &TenantCollectionsService{documents: documents, names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint((&DomainStorageService{documents: documents, names: &CollectionNamesService{}}).Insert(DomainDocument{DomainId: domainId, Name: domainId, State: state})))
	return domainId
}

func (test *MemberServiceTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *MemberServiceTest) relay(documents *DocumentsService) *OutboxRelayService {
	names := &CollectionNamesService{}
	store := &OutboxStoreService{domains: &DomainStorageService{documents: documents, names: names}, members: &MemberStorageService{documents: documents, names: names}, names: names}
	return &OutboxRelayService{store: store, settings: &SettingsService{}, domains: &DomainStorageService{documents: documents, names: names}}
}

func (test *MemberServiceTest) reset(domainId string) string {
	documents := &DocumentsService{dataSource: DataSource{}}
	names := &CollectionNamesService{}
	domains := &DomainStorageService{documents: documents, names: names}
	old, _, _ := domains.Find(domainId)
	domains.Delete(old)
	(&TenantCollectionsService{documents: documents, names: names}).Drop(domainId)
	return domainId
}

func (test *MemberServiceTest) lister() *MemberService {
	documents := &DocumentsService{dataSource: DataSource{}}
	people := &PeopleStorageService{documents: documents, names: &CollectionNamesService{}}
	return &MemberService{members: &MemberStorageService{documents: documents, names: &CollectionNamesService{}}, people: people, access: &PersonAccessService{people: people}}
}
