import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"

	"mesail.dev/packages/oidc"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type RecipientServiceTest struct {
	behaviors *BehaviorContext
}

func (test *RecipientServiceTest) TestMemberOfTheCallersDomainResolvesToChat() {
	service, caller, world := test.world()
	bo := test.resolve(service, caller, world["bo"])
	Assert.Equals("member", bo.Kind)
	Assert.Equals("chat", bo.DefaultDelivery)
	Assert.Equals("chat,email,both", strings.Join(bo.AllowedDeliveries, ","))
	Assert.Equals("Bo Member", bo.DisplayName)
	Assert.Equals("bo-id", bo.MemberId)
}

func (test *RecipientServiceTest) TestMemberOfAnotherHostedDomainIsNeverOfferedChat() {
	service, caller, world := test.world()
	eve := test.resolve(service, caller, world["eve"])
	Assert.Equals("otherDomain", eve.Kind)
	Assert.Equals("email", eve.DefaultDelivery)
	Assert.Equals("email", strings.Join(eve.AllowedDeliveries, ","))
}

func (test *RecipientServiceTest) TestMemberOfAnotherHostedDomainRevealsNoNameOrId() {
	service, caller, world := test.world()
	eve := test.resolve(service, caller, world["eve"])
	Assert.Equals("", eve.DisplayName)
	Assert.Equals("", eve.MemberId)
	Assert.Equals("", eve.ChannelId)
}

func (test *RecipientServiceTest) TestUnknownAddressOfAnotherHostedDomainIsOtherDomain() {
	service, caller, world := test.world()
	nobody := test.resolve(service, caller, "nobody@"+world["otherDomain"])
	Assert.Equals("otherDomain", nobody.Kind)
}

func (test *RecipientServiceTest) TestAddressThatIsNotHostedIsExternal() {
	service, caller, _ := test.world()
	carol := test.resolve(service, caller, "carol@partner.test")
	Assert.Equals("external", carol.Kind)
	Assert.Equals("email", strings.Join(carol.AllowedDeliveries, ","))
}

func (test *RecipientServiceTest) TestUnknownAddressOfTheCallersDomainIsExternal() {
	service, caller, world := test.world()
	nobody := test.resolve(service, caller, "nobody@"+world["domain"])
	Assert.Equals("external", nobody.Kind)
}

func (test *RecipientServiceTest) resolve(service *RecipientService, caller oidc.Caller, address string) ResolvedRecipient {
	response, err := service.Resolve(Admission{Caller: caller}, ResolveRequest{Addresses: []string{address}})
	Assert.IsTrue(err == nil, "resolve failed")
	Assert.Equals(1, len(response.Recipients))
	return response.Recipients[0]
}

func (test *RecipientServiceTest) world() (*RecipientService, oidc.Caller, map[string]string) {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{}}`)
	run := test.unique()
	domain := "one-" + run + ".test"
	otherDomain := "two-" + run + ".test"
	documents := &DocumentsService{dataSource: DataSource{}}
	domains := &DomainStorageService{documents: documents, names: &CollectionNamesService{}}
	members := &MemberStorageService{documents: documents, names: &CollectionNamesService{}}
	addresses := &AddressStorageService{documents: documents, names: &CollectionNamesService{}}
	tenants := &TenantService{domains: domains}
	memberService := &MemberService{tenants: tenants, members: members, addresses: addresses, events: &EventService{settings: &SettingsService{}}, relay: test.relay(documents)}
	test.host(domains, &TenantCollectionsService{documents: documents, names: &CollectionNamesService{}}, domain)
	test.host(domains, &TenantCollectionsService{documents: documents, names: &CollectionNamesService{}}, otherDomain)
	caller := oidc.Caller{DomainId: domain, MemberId: "ana-id", DisplayName: "Ana Admin", Email: "ana@" + domain}
	test.remember(memberService, caller)
	test.remember(memberService, oidc.Caller{DomainId: domain, MemberId: "bo-id", DisplayName: "Bo Member", Email: "bo@" + domain})
	test.remember(memberService, oidc.Caller{DomainId: otherDomain, MemberId: "eve-id", DisplayName: "Eve Other", Email: "eve@" + otherDomain})
	addresses.Insert(AddressDocument{Address: "bob@" + domain, DomainId: domain, Kind: "alias", TargetId: "bo-id"})
	addresses.Insert(AddressDocument{Address: "team@" + domain, DomainId: domain, Kind: "channel", TargetId: "ch_team"})
	service := &RecipientService{tenants: tenants, lookup: &RecipientLookupService{targets: &LocalTargetService{addresses: addresses, members: members}}}
	world := map[string]string{"domain": domain, "otherDomain": otherDomain, "bo": "bo@" + domain, "eve": "eve@" + otherDomain, "alias": "bob@" + domain, "channel": "team@" + domain}
	return service, caller, world
}

func (test *RecipientServiceTest) host(domains *DomainStorageService, collections *TenantCollectionsService, domainId string) {
	Assert.IsTrue(collections.Prepare(domainId) == nil, "prepare failed")
	Assert.IsTrue(domains.Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"}) == nil, "insert failed")
}

func (test *RecipientServiceTest) remember(service *MemberService, caller oidc.Caller) {
	_, err := service.Remember(caller)
	Assert.IsTrue(err == nil, "remember failed")
}

func (test *RecipientServiceTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *RecipientServiceTest) relay(documents *DocumentsService) *OutboxRelayService {
	names := &CollectionNamesService{}
	store := &OutboxStoreService{domains: &DomainStorageService{documents: documents, names: names}, members: &MemberStorageService{documents: documents, names: names}, names: names}
	return &OutboxRelayService{store: store, settings: &SettingsService{}, domains: &DomainStorageService{documents: documents, names: names}}
}
