// @Test(SessionTestSetup, {"partition":"tests"})
type SessionRealmServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionRealmServiceTest) TestDomainOfLowersAndTrimsTheAddress() {
	Assert.Equals("example.com", (&SessionRealmService{}).DomainOf("  Ana@Example.COM "))
}

func (test *SessionRealmServiceTest) TestDomainOfRefusesAnAddressWithoutDomain() {
	Assert.Equals("", (&SessionRealmService{}).DomainOf("ana"))
}

func (test *SessionRealmServiceTest) TestDomainOfRefusesAHostWithoutDot() {
	Assert.Equals("", (&SessionRealmService{}).DomainOf("ana@localhost"))
}

func (test *SessionRealmServiceTest) TestDomainOfRefusesSpaces() {
	Assert.Equals("", (&SessionRealmService{}).DomainOf("ana ruiz@example.com"))
}

func (test *SessionRealmServiceTest) TestDomainOfKeepsSubdomains() {
	Assert.Equals("lab.example.net", (&SessionRealmService{}).DomainOf("bo@lab.example.net"))
}
