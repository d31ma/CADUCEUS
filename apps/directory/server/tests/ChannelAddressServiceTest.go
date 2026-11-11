import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"mesail.dev/packages/oidc"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type ChannelAddressServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ChannelAddressServiceTest) TestAChannelAddressIsStored() {
	service, admission, domain := test.world()
	address, err := service.Add(admission, ChannelAddressRequest{Address: "Team@" + domain, ChannelId: "ch_team"})
	route, found, _ := service.addresses.Find("team@" + domain)
	Assert.IsTrue(err == nil, "add failed")
	Assert.Equals("channel", address.Kind)
	Assert.Equals("ch_team", address.TargetId)
	Assert.IsTrue(found, "address not stored")
	Assert.Equals("ch_team", route.TargetId)
}

func (test *ChannelAddressServiceTest) TestRepeatingTheSameChannelAddressIsIdempotent() {
	service, admission, domain := test.world()
	service.Add(admission, ChannelAddressRequest{Address: "team@" + domain, ChannelId: "ch_team"})
	_, err := service.Add(admission, ChannelAddressRequest{Address: "team@" + domain, ChannelId: "ch_team"})
	Assert.IsTrue(err == nil, "repeat refused")
}

func (test *ChannelAddressServiceTest) TestAnAddressOfAnotherChannelIsADuplicate() {
	service, admission, domain := test.world()
	service.Add(admission, ChannelAddressRequest{Address: "team@" + domain, ChannelId: "ch_team"})
	_, err := service.Add(admission, ChannelAddressRequest{Address: "team@" + domain, ChannelId: "ch_other"})
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *ChannelAddressServiceTest) TestAMemberAddressIsADuplicate() {
	service, admission, domain := test.world()
	_, err := service.Add(admission, ChannelAddressRequest{Address: "ana@" + domain, ChannelId: "ch_team"})
	var duplicate *Duplicate
	Assert.IsTrue(errors.As(err, &duplicate), "want Duplicate")
}

func (test *ChannelAddressServiceTest) TestAnAddressInAnotherDomainIsRefused() {
	service, admission, _ := test.world()
	_, err := service.Add(admission, ChannelAddressRequest{Address: "team@example.org", ChannelId: "ch_team"})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("address", invalid.Field)
}

func (test *ChannelAddressServiceTest) TestAChannelIdIsRequired() {
	service, admission, domain := test.world()
	_, err := service.Add(admission, ChannelAddressRequest{Address: "team@" + domain})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("channelId", invalid.Field)
}

func (test *ChannelAddressServiceTest) TestARemovedChannelAddressIsGone() {
	service, admission, domain := test.world()
	service.Add(admission, ChannelAddressRequest{Address: "team@" + domain, ChannelId: "ch_team"})
	err := service.Remove(admission, "team@"+domain)
	_, found, _ := service.addresses.Find("team@" + domain)
	Assert.IsTrue(err == nil, "remove failed")
	Assert.IsTrue(!found, "address still stored")
}

func (test *ChannelAddressServiceTest) TestAMemberAddressCannotBeRemovedAsAChannelAddress() {
	service, admission, domain := test.world()
	err := service.Remove(admission, "ana@"+domain)
	_, found, _ := service.addresses.Find("ana@" + domain)
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
	Assert.IsTrue(found, "member address was removed")
}

func (test *ChannelAddressServiceTest) TestARefusedCallerChangesNothing() {
	service, admission, domain := test.world()
	admission.Refusal = &Unauthenticated{}
	_, addErr := service.Add(admission, ChannelAddressRequest{Address: "team@" + domain, ChannelId: "ch_team"})
	removeErr := service.Remove(admission, "ana@"+domain)
	_, found, _ := service.addresses.Find("team@" + domain)
	var refused *Unauthenticated
	Assert.IsTrue(errors.As(addErr, &refused), "add: want Unauthenticated")
	Assert.IsTrue(errors.As(removeErr, &refused), "remove: want Unauthenticated")
	Assert.IsTrue(!found, "address stored for a refused caller")
}

func (test *ChannelAddressServiceTest) world() (*ChannelAddressService, Admission, string) {
	domainId := test.reset("channel-unit.test")
	documents := &DocumentsService{dataSource: DataSource{}}
	names := &CollectionNamesService{}
	addresses := &AddressStorageService{documents: documents, names: names}
	Assert.IsTrue((&TenantCollectionsService{documents: documents, names: names}).Prepare(domainId) == nil, "prepare failed")
	Assert.IsTrue(addresses.Insert(AddressDocument{Address: "ana@" + domainId, DomainId: domainId, Kind: "member", TargetId: "ana-id"}) == nil, "insert failed")
	admission := Admission{Caller: oidc.Caller{DomainId: domainId, MemberId: "ana-id", Roles: []string{"DomainAdmin"}}}
	return &ChannelAddressService{addresses: addresses, rules: &AddressRulesService{}}, admission, domainId
}

func (test *ChannelAddressServiceTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *ChannelAddressServiceTest) reset(domainId string) string {
	documents := &DocumentsService{dataSource: DataSource{}}
	names := &CollectionNamesService{}
	domains := &DomainStorageService{documents: documents, names: names}
	old, _, _ := domains.Find(domainId)
	domains.Delete(old)
	(&TenantCollectionsService{documents: documents, names: names}).Drop(domainId)
	return domainId
}
