// PresenceServiceTest calls every method of PresenceService on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(PresenceTestSetup)
type PresenceServiceTest struct {
	behaviors *BehaviorContext
}

func (test *PresenceServiceTest) TestHeartbeatNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&PresenceService{}).Heartbeat("ada@example.com", []string{"example.com"})
		return err
	})
}

func (test *PresenceServiceTest) TestSnapshotForDomainsNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&PresenceService{}).SnapshotForDomains([]string{"example.com"})
		return err
	})
}

func (test *PresenceServiceTest) TestIsOnlineNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&PresenceService{}).IsOnline("ada@example.com")
		return err
	})
}
