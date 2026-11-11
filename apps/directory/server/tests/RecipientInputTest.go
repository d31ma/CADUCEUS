import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"

	"mesail.dev/packages/oidc"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type RecipientInputTest struct {
	behaviors *BehaviorContext
}

func (test *RecipientInputTest) TestAddressesAreMatchedWithoutCase() {
	service, caller, world := test.world()
	bo := test.resolve(service, caller, strings.ToUpper(world["bo"]))
	Assert.Equals(world["bo"], bo.Address)
	Assert.Equals("member", bo.Kind)
}

func (test *RecipientInputTest) TestAliasResolvesToItsMember() {
	service, caller, world := test.world()
	alias := test.resolve(service, caller, world["alias"])
	Assert.Equals("member", alias.Kind)
	Assert.Equals("bo-id", alias.MemberId)
}

func (test *RecipientInputTest) TestChannelAddressResolvesToChatOnly() {
	service, caller, world := test.world()
	channel := test.resolve(service, caller, world["channel"])
	Assert.Equals("channel", channel.Kind)
	Assert.Equals("chat", strings.Join(channel.AllowedDeliveries, ","))
	Assert.Equals("ch_team", channel.ChannelId)
}

func (test *RecipientInputTest) TestRepeatedAddressesAreResolvedOnce() {
	service, caller, world := test.world()
	response, err := service.Resolve(Admission{Caller: caller}, ResolveRequest{Addresses: []string{world["bo"], strings.ToUpper(world["bo"])}})
	Assert.IsTrue(err == nil, "resolve failed")
	Assert.Equals(1, len(response.Recipients))
}

func (test *RecipientInputTest) TestInvalidAddressIsRefused() {
	service, caller, _ := test.world()
	_, err := service.Resolve(Admission{Caller: caller}, ResolveRequest{Addresses: []string{"not an address"}})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("addresses", invalid.Field)
}

func (test *RecipientInputTest) TestMoreThanOneHundredAddressesAreRefused() {
	service, caller, _ := test.world()
	addresses := strings.Split(strings.Repeat("a@partner.test,", 101), ",")
	_, err := service.Resolve(Admission{Caller: caller}, ResolveRequest{Addresses: addresses[:101]})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
}

func (test *RecipientInputTest) resolve(service *RecipientService, caller oidc.Caller, address string) ResolvedRecipient {
	response, err := service.Resolve(Admission{Caller: caller}, ResolveRequest{Addresses: []string{address}})
	Assert.IsTrue(err == nil, "resolve failed")
	Assert.Equals(1, len(response.Recipients))
	return response.Recipients[0]
}

func (test *RecipientInputTest) world() (*RecipientService, oidc.Caller, map[string]string) {
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

func (test *RecipientInputTest) host(domains *DomainStorageService, collections *TenantCollectionsService, domainId string) {
	Assert.IsTrue(collections.Prepare(domainId) == nil, "prepare failed")
	Assert.IsTrue(domains.Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"}) == nil, "insert failed")
}

func (test *RecipientInputTest) remember(service *MemberService, caller oidc.Caller) {
	_, err := service.Remember(caller)
	Assert.IsTrue(err == nil, "remember failed")
}

func (test *RecipientInputTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *RecipientInputTest) relay(documents *DocumentsService) *OutboxRelayService {
	names := &CollectionNamesService{}
	store := &OutboxStoreService{domains: &DomainStorageService{documents: documents, names: names}, members: &MemberStorageService{documents: documents, names: names}, names: names}
	return &OutboxRelayService{store: store, settings: &SettingsService{}, domains: &DomainStorageService{documents: documents, names: names}}
}
