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

func (test *MemberServiceTest) TestRememberStoresTheMemberFromTheToken() {
	service, storage, domain := test.world()
	member, err := service.Remember(test.caller(domain, "Bo Member"))
	stored, found, _ := storage.Find(domain, "bo-id")
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.IsTrue(found, "member not stored")
	Assert.Equals("Bo Member", stored.DisplayName)
	Assert.Equals("bo@"+domain, member.PrimaryAddress)
	Assert.Equals("active", stored.State)
}

func (test *MemberServiceTest) TestRememberRegistersThePrimaryAddress() {
	service, _, domain := test.world()
	service.Remember(test.caller(domain, "Bo Member"))
	addresses := &AddressStorageService{documents: &DocumentsService{dataSource: DataSource{}}, names: &CollectionNamesService{}}
	route, found, _ := addresses.Find("bo@" + domain)
	Assert.IsTrue(found, "address not registered")
	Assert.Equals("member", route.Kind)
	Assert.Equals("bo-id", route.TargetId)
}

func (test *MemberServiceTest) TestAnEmailOfAnotherDomainIsNotTheMembersAddress() {
	service, _, domain := test.world()
	caller := oidc.Caller{DomainId: domain, MemberId: "bo-id", DisplayName: "", Email: "bo@elsewhere.test"}
	member, err := service.Remember(caller)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("", member.PrimaryAddress)
	Assert.Equals("bo@elsewhere.test", member.DisplayName)
}

func (test *MemberServiceTest) TestAMemberOfADomainThatIsNotHostedIsRefused() {
	service, _, _ := test.world()
	_, err := service.Remember(test.caller("unknown-"+test.unique()+".test", "Bo"))
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
}

func (test *MemberServiceTest) TestAMemberOfADisabledDomainIsRefused() {
	service, _, _ := test.world()
	domain := test.hosted("member-disabled.test", "disabled")
	_, err := service.Remember(test.caller(domain, "Bo"))
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
}

func (test *MemberServiceTest) TestSearchFindsMembersByName() {
	service, _, domain := test.world()
	service.Remember(test.caller(domain, "Bo Member"))
	service.Remember(oidc.Caller{DomainId: domain, MemberId: "ana-id", DisplayName: "Ana Admin", Email: "ana@" + domain})
	page, err := service.Search(Admission{Caller: test.caller(domain, "Bo Member")}, MemberQuery{Text: "ana"}, nil)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(1, len(page.Items))
	Assert.Equals("ana-id", page.Items[0].MemberId)
	Assert.Equals("", page.NextCursor)
}

func (test *MemberServiceTest) TestSearchNeverReturnsAnotherDomain() {
	service, _, domain := test.world()
	otherDomain := test.hosted("member-other.test", "active")
	service.Remember(oidc.Caller{DomainId: otherDomain, MemberId: "eve-id", DisplayName: "Eve", Email: "eve@" + otherDomain})
	page, err := service.Search(Admission{Caller: test.caller(domain, "Bo Member")}, MemberQuery{Text: "eve"}, nil)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(0, len(page.Items))
}

func (test *MemberServiceTest) TestSearchRefusesALimitAboveOneHundred() {
	service, _, domain := test.world()
	_, err := service.Search(Admission{Caller: test.caller(domain, "Bo")}, MemberQuery{Limit: 101}, nil)
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("limit", invalid.Field)
}

func (test *MemberServiceTest) TestSearchPagesWithACursor() {
	service, _, domain := test.world()
	service.Remember(test.caller(domain, "Bo Member"))
	service.Remember(oidc.Caller{DomainId: domain, MemberId: "ana-id", DisplayName: "Ana Admin", Email: "ana@" + domain})
	first, _ := service.Search(Admission{Caller: test.caller(domain, "Bo Member")}, MemberQuery{Limit: 1}, nil)
	second, err := service.Search(Admission{Caller: test.caller(domain, "Bo Member")}, MemberQuery{Limit: 1, Cursor: first.NextCursor}, nil)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("1", first.NextCursor)
	Assert.Equals(1, len(second.Items))
	Assert.Equals("", second.NextCursor)
}

func (test *MemberServiceTest) TestSearchRefusesACursorItDidNotGiveOut() {
	service, _, domain := test.world()
	_, err := service.Search(Admission{Caller: test.caller(domain, "Bo")}, MemberQuery{Cursor: "abc"}, nil)
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("cursor", invalid.Field)
}

func (test *MemberServiceTest) TestSummaryOfAMemberOfTheCallersDomain() {
	service, _, domain := test.world()
	service.Remember(oidc.Caller{DomainId: domain, MemberId: "ana-id", DisplayName: "Ana Admin", Email: "ana@" + domain})
	summary, err := service.Summary(Admission{Caller: test.caller(domain, "Bo")}, "ana-id")
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("Ana Admin", summary.DisplayName)
	Assert.Equals("ana@"+domain, summary.PrimaryAddress)
}

func (test *MemberServiceTest) TestSummaryOfAMemberOfAnotherDomainIsNotFound() {
	service, _, domain := test.world()
	otherDomain := test.hosted("member-other.test", "active")
	service.Remember(oidc.Caller{DomainId: otherDomain, MemberId: "eve-id", DisplayName: "Eve", Email: "eve@" + otherDomain})
	_, err := service.Summary(Admission{Caller: test.caller(domain, "Bo")}, "eve-id")
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *MemberServiceTest) TestSummaryForARefusedCallerIsTheRefusal() {
	service, _, domain := test.world()
	_, err := service.Summary(Admission{Caller: test.caller(domain, "Bo"), Refusal: &Unauthenticated{}}, "bo-id")
	var refused *Unauthenticated
	Assert.IsTrue(errors.As(err, &refused), "want Unauthenticated")
}

func (test *MemberServiceTest) caller(domain string, name string) oidc.Caller {
	return oidc.Caller{DomainId: domain, MemberId: "bo-id", DisplayName: name, Email: "Bo@" + strings.ToUpper(domain)}
}

func (test *MemberServiceTest) world() (*MemberService, *MemberStorageService, string) {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{}}`)
	documents := &DocumentsService{dataSource: DataSource{}}
	members := &MemberStorageService{documents: documents, names: &CollectionNamesService{}}
	addresses := &AddressStorageService{documents: documents, names: &CollectionNamesService{}}
	tenants := &TenantService{domains: &DomainStorageService{documents: documents, names: &CollectionNamesService{}}}
	return &MemberService{tenants: tenants, members: members, addresses: addresses, events: &EventService{settings: &SettingsService{}}, relay: test.relay(documents)}, members, test.hosted("member-unit.test", "active")
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
