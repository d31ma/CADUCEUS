import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type AddressServiceTest struct {
	behaviors *BehaviorContext
}

func (test *AddressServiceTest) TestAHostedAddressIsFound() {
	service, domain := test.world()
	address, err := service.Find("Team@" + domain)
	Assert.IsTrue(err == nil, "find failed")
	Assert.Equals("team@"+domain, address.Address)
	Assert.Equals("channel", address.Kind)
	Assert.Equals("ch_team", address.TargetId)
	Assert.Equals(domain, address.DomainId)
}

func (test *AddressServiceTest) TestAnUnknownAddressOfAHostedDomainIsNotFound() {
	service, domain := test.world()
	_, err := service.Find("nobody@" + domain)
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *AddressServiceTest) TestAnAddressOfADomainThatIsNotHostedIsDomainNotHosted() {
	service, _ := test.world()
	_, err := service.Find("carol@partner-" + test.unique() + ".test")
	var notHosted *DomainNotHosted
	Assert.IsTrue(errors.As(err, &notHosted), "want DomainNotHosted")
}

func (test *AddressServiceTest) TestAnAddressOfAPendingDomainIsDomainNotHosted() {
	service, _ := test.world()
	pending := "p-" + test.unique() + ".test"
	Assert.IsTrue(service.domains.Insert(DomainDocument{DomainId: pending, Name: pending, State: "pending"}) == nil, "insert failed")
	_, err := service.Find("team@" + pending)
	var notHosted *DomainNotHosted
	Assert.IsTrue(errors.As(err, &notHosted), "want DomainNotHosted")
}

func (test *AddressServiceTest) TestAnUnknownAddressOfAnActiveDomainIsNotDomainNotHosted() {
	service, domain := test.world()
	_, err := service.Find("nobody@" + domain)
	var notHosted *DomainNotHosted
	Assert.IsTrue(!errors.As(err, &notHosted), "an active domain is hosted")
}

func (test *AddressServiceTest) TestAnInvalidAddressIsNotFound() {
	service, _ := test.world()
	_, err := service.Find("not an address")
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *AddressServiceTest) world() (*AddressService, string) {
	domainId := "s-" + test.unique() + ".test"
	documents := &DocumentsService{dataSource: DataSource{}}
	addresses := &AddressStorageService{documents: documents, names: &CollectionNamesService{}}
	domains := &DomainStorageService{documents: documents, names: &CollectionNamesService{}}
	Assert.IsTrue((&TenantCollectionsService{documents: documents, names: &CollectionNamesService{}}).Prepare(domainId) == nil, "prepare failed")
	Assert.IsTrue(domains.Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"}) == nil, "insert failed")
	Assert.IsTrue(addresses.Insert(AddressDocument{Address: "team@" + domainId, DomainId: domainId, Kind: "channel", TargetId: "ch_team"}) == nil, "insert failed")
	return &AddressService{domains: domains, addresses: addresses}, domainId
}

func (test *AddressServiceTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
