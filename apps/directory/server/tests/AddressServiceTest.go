import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type AddressServiceTest struct {
	behaviors *BehaviorContext
}

func (test *AddressServiceTest) TestAHostedAddressIsFound() {
	service, domain := test.world()
	address, err := service.Find("Team@" + domain)
	Assert.Equals("<nil>", fmt.Sprint(err))
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
	pending := test.reset("address-pending.test")
	Assert.Equals("<nil>", fmt.Sprint(service.domains.Insert(DomainDocument{DomainId: pending, Name: pending, State: "pending"})))
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
	domainId := test.reset("address-unit.test")
	documents := &DocumentsService{dataSource: DataSource{}}
	addresses := &AddressStorageService{documents: documents, names: &CollectionNamesService{}}
	domains := &DomainStorageService{documents: documents, names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint((&TenantCollectionsService{documents: documents, names: &CollectionNamesService{}}).Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(domains.Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
	Assert.Equals("<nil>", fmt.Sprint(addresses.Insert(AddressDocument{Address: "team@" + domainId, DomainId: domainId, Kind: "channel", TargetId: "ch_team"})))
	return &AddressService{domains: domains, addresses: addresses}, domainId
}

func (test *AddressServiceTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func (test *AddressServiceTest) reset(domainId string) string {
	documents := &DocumentsService{dataSource: DataSource{}}
	names := &CollectionNamesService{}
	domains := &DomainStorageService{documents: documents, names: names}
	old, _, _ := domains.Find(domainId)
	domains.Delete(old)
	(&TenantCollectionsService{documents: documents, names: names}).Drop(domainId)
	return domainId
}
