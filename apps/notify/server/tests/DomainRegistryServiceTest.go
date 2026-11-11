import "slices"

// @Test(NotifyTestSetup, {"partition":"tests"})
type DomainRegistryServiceTest struct {
	behaviors *BehaviorContext
}

func (test *DomainRegistryServiceTest) TestRegisterIsSafeToRepeat() {
	domainId := NotifyTestSetup{}.Domain()
	registry := NotifyTestSetup{}.Domains()
	err := registry.Register(domainId)
	again := registry.Register(domainId)
	Assert.IsTrue(err == nil && again == nil, "register failed")
}

func (test *DomainRegistryServiceTest) TestAllListsARegisteredDomain() {
	domainId := NotifyTestSetup{}.Domain()
	registry := NotifyTestSetup{}.Domains()
	registry.Register(domainId)
	domains, err := registry.All(100000)
	Assert.IsTrue(err == nil, "list failed")
	Assert.IsTrue(slices.Contains(domains, domainId), "domain missing")
}
