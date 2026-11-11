import "errors"

// @Test(JobsTestSetup, {"partition":"tests"})
type PresenceServiceTest struct {
	behaviors *BehaviorContext
}

func (test *PresenceServiceTest) TestStatesReadsTheRealtimeAnswer() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Keycloak(setup.Realms(domainId))
	server := JobsTestSetup{}.PresenceServer(map[string]string{"member-1": "active"}, 200)
	JobsTestSetup{}.UseNotify([]string{"example.com"}, server.URL)
	states, err := JobsTestSetup{}.Presence().States(domainId, []string{"member-1", "member-2"})
	Assert.IsTrue(err == nil, "presence failed")
	Assert.Equals("active", states["member-1"].State)
	Assert.Equals("", states["member-2"].State)
}

func (test *PresenceServiceTest) TestStatesIsEmptyWithoutAUrl() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	states, err := JobsTestSetup{}.Presence().States(domainId, []string{"member-1"})
	Assert.IsTrue(err == nil, "presence failed")
	Assert.Equals(0, len(states))
}

func (test *PresenceServiceTest) TestStatesReportsAnUnavailableService() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Keycloak(setup.Realms(domainId))
	server := JobsTestSetup{}.PresenceServer(map[string]string{}, 500)
	JobsTestSetup{}.UseNotify([]string{"example.com"}, server.URL)
	_, err := JobsTestSetup{}.Presence().States(domainId, []string{"member-1"})
	unavailable := &PresenceUnavailable{}
	Assert.IsTrue(errors.As(err, &unavailable))
	Assert.Equals(500, unavailable.Status)
}
