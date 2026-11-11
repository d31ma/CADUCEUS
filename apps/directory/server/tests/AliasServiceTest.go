import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"mesail.dev/packages/oidc"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type AliasServiceTest struct {
	behaviors *BehaviorContext
}

func (test *AliasServiceTest) TestAnAliasPointsAtTheNamedMember() {
	service, caller, domain := test.world()
	alias, err := service.Add(Admission{Caller: caller}, CreateAliasRequest{Address: "Sales@" + domain, MemberId: "bo-id"})
	Assert.IsTrue(err == nil, "add failed")
	Assert.Equals("sales@"+domain, alias.Address)
	Assert.Equals("alias", alias.Kind)
	Assert.Equals("bo-id", alias.TargetId)
	Assert.Equals(domain, alias.DomainId)
}

func (test *AliasServiceTest) TestAnAliasWithoutAMemberPointsAtTheCaller() {
	service, caller, domain := test.world()
	alias, _ := service.Add(Admission{Caller: caller}, CreateAliasRequest{Address: "boss@" + domain})
	Assert.Equals("ana-id", alias.TargetId)
}

func (test *AliasServiceTest) TestAnAliasInAnotherDomainIsRefused() {
	service, caller, _ := test.world()
	_, err := service.Add(Admission{Caller: caller}, CreateAliasRequest{Address: "sales@example.org"})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("address", invalid.Field)
}

func (test *AliasServiceTest) TestAMemberOfAnotherDomainIsNotFound() {
	service, caller, domain := test.world()
	_, err := service.Add(Admission{Caller: caller}, CreateAliasRequest{Address: "x@" + domain, MemberId: "eve-id"})
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *AliasServiceTest) TestAnAddressInUseIsADuplicate() {
	service, caller, domain := test.world()
	_, err := service.Add(Admission{Caller: caller}, CreateAliasRequest{Address: "bo@" + domain})
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *AliasServiceTest) TestARefusedCallerAddsNothing() {
	service, caller, domain := test.world()
	_, err := service.Add(Admission{Caller: caller, Refusal: &Unauthenticated{}}, CreateAliasRequest{Address: "refused@" + domain})
	_, taken, _ := service.addresses.Find("refused@" + domain)
	var refused *Unauthenticated
	Assert.IsTrue(errors.As(err, &refused), "want Unauthenticated")
	Assert.IsTrue(!taken, "alias stored for a refused caller")
}

func (test *AliasServiceTest) world() (*AliasService, oidc.Caller, string) {
	domainId := "a-" + test.unique() + ".test"
	documents := &DocumentsService{dataSource: DataSource{}}
	members := &MemberStorageService{documents: documents, names: &CollectionNamesService{}}
	addresses := &AddressStorageService{documents: documents, names: &CollectionNamesService{}}
	domains := &DomainStorageService{documents: documents, names: &CollectionNamesService{}}
	Assert.IsTrue((&TenantCollectionsService{documents: documents, names: &CollectionNamesService{}}).Prepare(domainId) == nil, "prepare failed")
	Assert.IsTrue(domains.Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"}) == nil, "insert failed")
	memberService := &MemberService{tenants: &TenantService{domains: domains}, members: members, addresses: addresses, events: &EventService{settings: &SettingsService{}}, relay: test.relay(documents)}
	caller := oidc.Caller{DomainId: domainId, MemberId: "ana-id", DisplayName: "Ana", Email: "ana@" + domainId}
	memberService.Remember(caller)
	memberService.Remember(oidc.Caller{DomainId: domainId, MemberId: "bo-id", DisplayName: "Bo", Email: "bo@" + domainId})
	return &AliasService{addresses: addresses, members: members}, caller, domainId
}

func (test *AliasServiceTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *AliasServiceTest) relay(documents *DocumentsService) *OutboxRelayService {
	names := &CollectionNamesService{}
	store := &OutboxStoreService{domains: &DomainStorageService{documents: documents, names: names}, members: &MemberStorageService{documents: documents, names: names}, names: names}
	return &OutboxRelayService{store: store, settings: &SettingsService{}, domains: &DomainStorageService{documents: documents, names: names}}
}
