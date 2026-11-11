import (
	"errors"
	"net/http/httptest"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type ControllersTest struct {
	behaviors *BehaviorContext
}

func (test *ControllersTest) TestResolveRefusesARequestWithoutAValidToken() {
	documents := &DocumentsService{dataSource: DataSource{}}
	recipients := &RecipientService{tenants: &TenantService{domains: &DomainStorageService{documents: documents, names: &CollectionNamesService{}}}}
	controller := &RecipientsController{admission: &AdmissionService{callers: &CallerService{}}, recipients: recipients}
	_, err := controller.PostResolve("Bearer forged", ResolveRequest{Addresses: []string{"bo@example.com"}})
	Assert.IsTrue(test.unauthenticated(err), "want Unauthenticated")
}

func (test *ControllersTest) TestMembersRefusesARequestWithoutAValidToken() {
	controller := &MembersController{admission: &AdmissionService{callers: &CallerService{}}}
	_, err := controller.GetMembers(httptest.NewRequest("GET", "/api/v1/directory/members?query=bo&limit=10", nil))
	Assert.IsTrue(test.unauthenticated(err), "want Unauthenticated")
}

func (test *ControllersTest) TestAliasesRefusesARequestWithoutAValidToken() {
	controller := &AliasesController{admission: &AdmissionService{callers: &CallerService{}}}
	_, err := controller.PostAlias("Bearer forged", CreateAliasRequest{Address: "sales@example.com"})
	Assert.IsTrue(test.unauthenticated(err), "want Unauthenticated")
}

func (test *ControllersTest) TestDomainRoutesRefuseARequestWithoutAValidToken() {
	controller := &DomainsController{callers: &CallerService{}}
	_, listErr := controller.GetDomains("Bearer forged")
	_, addErr := controller.PostDomain("Bearer forged", CreateDomainRequest{Name: "evil.example"})
	_, verifyErr := controller.PostVerify("Bearer forged", "example.com")
	_, deleteErr := controller.DeleteDomain("Bearer forged", "example.com")
	Assert.IsTrue(test.unauthenticated(listErr), "list: want Unauthenticated")
	Assert.IsTrue(test.unauthenticated(addErr), "add: want Unauthenticated")
	Assert.IsTrue(test.unauthenticated(verifyErr), "verify: want Unauthenticated")
	Assert.IsTrue(test.unauthenticated(deleteErr), "delete: want Unauthenticated")
}

func (test *ControllersTest) TestGetDomainRefusesARequestWithoutAValidToken() {
	documents := &DocumentsService{dataSource: DataSource{}}
	controller := &DomainsController{callers: &CallerService{}, domains: &DomainService{domains: &DomainStorageService{documents: documents, names: &CollectionNamesService{}}}}
	_, err := controller.GetDomain("Bearer forged", "never-hosted.invalid")
	Assert.IsTrue(test.unauthenticated(err), "want Unauthenticated")
}

func (test *ControllersTest) TestGetAddressRefusesARequestWithoutAValidToken() {
	documents := &DocumentsService{dataSource: DataSource{}}
	controller := &AddressesController{callers: &CallerService{}, addresses: &AddressService{domains: &DomainStorageService{documents: documents, names: &CollectionNamesService{}}, addresses: &AddressStorageService{documents: documents, names: &CollectionNamesService{}}}}
	_, err := controller.GetAddress("Bearer forged", "carol@never-hosted.invalid")
	Assert.IsTrue(test.unauthenticated(err), "want Unauthenticated")
}

func (test *ControllersTest) TestMemberLookupRefusesARequestWithoutAValidToken() {
	controller := &MembersController{admission: &AdmissionService{callers: &CallerService{}}}
	_, err := controller.GetMember("Bearer forged", "bo-id")
	Assert.IsTrue(test.unauthenticated(err), "want Unauthenticated")
}

func (test *ControllersTest) TestChannelAddressRoutesRefuseARequestWithoutAValidToken() {
	controller := &ChannelAddressesController{admission: &AdmissionService{callers: &CallerService{}}, channels: &ChannelAddressService{rules: &AddressRulesService{}}}
	_, addErr := controller.PostChannelAddress("Bearer forged", ChannelAddressRequest{Address: "team@example.com", ChannelId: "ch_1"})
	deleteErr := controller.DeleteChannelAddress("Bearer forged", "team@example.com")
	Assert.IsTrue(test.unauthenticated(addErr), "add: want Unauthenticated")
	Assert.IsTrue(test.unauthenticated(deleteErr), "delete: want Unauthenticated")
}

func (test *ControllersTest) unauthenticated(err error) bool {
	var refused *Unauthenticated
	return errors.As(err, &refused)
}
