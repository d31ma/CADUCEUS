import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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

func (test *RecipientServiceTest) TestMemberOfAPendingDomainIsExternal() {
	service, caller, _ := test.world()
	carol := test.resolve(service, caller, test.memberOfDomainIn("pending"))
	Assert.Equals("external", carol.Kind)
	Assert.Equals("email", carol.DefaultDelivery)
	Assert.Equals("", carol.MemberId)
}

func (test *RecipientServiceTest) TestMemberOfADisabledDomainIsExternal() {
	service, caller, _ := test.world()
	carol := test.resolve(service, caller, test.memberOfDomainIn("disabled"))
	Assert.Equals("external", carol.Kind)
}

func (test *RecipientServiceTest) TestMemberOfARemovingDomainIsExternal() {
	service, caller, _ := test.world()
	carol := test.resolve(service, caller, test.memberOfDomainIn("removing"))
	Assert.Equals("external", carol.Kind)
}

func (test *RecipientServiceTest) memberOfDomainIn(state string) string {
	domainId := test.reset(state + "-service.test")
	documents := &DocumentsService{dataSource: DataSource{}}
	names := &CollectionNamesService{}
	Assert.Equals("<nil>", fmt.Sprint((&TenantCollectionsService{documents: documents, names: names}).Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint((&DomainStorageService{documents: documents, names: names}).Insert(DomainDocument{DomainId: domainId, Name: domainId, State: state})))
	Assert.Equals("<nil>", fmt.Sprint((&AddressStorageService{documents: documents, names: names}).Insert(AddressDocument{Address: "carol@" + domainId, DomainId: domainId, Kind: "member", TargetId: "carol-id"})))
	return "carol@" + domainId
}

func (test *RecipientServiceTest) resolve(service *RecipientService, caller oidc.Caller, address string) ResolvedRecipient {
	response, err := service.Resolve(Admission{Caller: caller}, ResolveRequest{Addresses: []string{address}})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(1, len(response.Recipients))
	return response.Recipients[0]
}

func (test *RecipientServiceTest) world() (*RecipientService, oidc.Caller, map[string]string) {
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{}}`)
	domain := test.reset("one-service.test")
	otherDomain := test.reset("two-service.test")
	documents := &DocumentsService{dataSource: DataSource{}}
	domains := &DomainStorageService{documents: documents, names: &CollectionNamesService{}}
	members := &MemberStorageService{documents: documents, names: &CollectionNamesService{}}
	addresses := &AddressStorageService{documents: documents, names: &CollectionNamesService{}}
	tenants := &TenantService{domains: domains}
	memberService := &MemberWriterService{members: members, addresses: addresses, events: &EventService{settings: &SettingsService{}}, relay: test.relay(documents)}
	test.host(domains, &TenantCollectionsService{documents: documents, names: &CollectionNamesService{}}, domain)
	test.host(domains, &TenantCollectionsService{documents: documents, names: &CollectionNamesService{}}, otherDomain)
	caller := oidc.Caller{DomainId: domain, MemberId: "ana-id", DisplayName: "Ana Admin", Email: "ana@" + domain}
	test.join(memberService, domain, "ana-id", "ana", "Ana Admin")
	test.join(memberService, domain, "bo-id", "bo", "Bo Member")
	test.join(memberService, otherDomain, "eve-id", "eve", "Eve Other")
	addresses.Insert(AddressDocument{Address: "bob@" + domain, DomainId: domain, Kind: "alias", TargetId: "bo-id"})
	addresses.Insert(AddressDocument{Address: "team@" + domain, DomainId: domain, Kind: "channel", TargetId: "ch_team"})
	service := &RecipientService{tenants: tenants, lookup: &RecipientLookupService{targets: &LocalTargetService{addresses: addresses, members: members}}}
	world := map[string]string{"domain": domain, "otherDomain": otherDomain, "bo": "bo@" + domain, "eve": "eve@" + otherDomain, "alias": "bob@" + domain, "channel": "team@" + domain}
	return service, caller, world
}

func (test *RecipientServiceTest) host(domains *DomainStorageService, collections *TenantCollectionsService, domainId string) {
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(domains.Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
}

func (test *RecipientServiceTest) join(writer *MemberWriterService, domainId string, personId string, username string, name string) {
	_, err := writer.Join(PersonDocument{PersonId: personId, Username: username, DisplayName: name, State: "active"}, domainId, false)
	Assert.Equals("<nil>", fmt.Sprint(err))
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

func (test *RecipientServiceTest) reset(domainId string) string {
	documents := &DocumentsService{dataSource: DataSource{}}
	names := &CollectionNamesService{}
	domains := &DomainStorageService{documents: documents, names: names}
	old, _, _ := domains.Find(domainId)
	domains.Delete(old)
	(&TenantCollectionsService{documents: documents, names: names}).Drop(domainId)
	return domainId
}
