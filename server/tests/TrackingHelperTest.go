// TrackingHelperTest calls every method of TrackingHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(TrackingTestSetup)
type TrackingHelperTest struct {
	behaviors *BehaviorContext
}

func (test *TrackingHelperTest) TestRecordOpenNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&TrackingHelper{}).RecordOpen("tracking-id-0123456789", "bob@example.com")
		return err
	})
}

func (test *TrackingHelperTest) TestRecordClickNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&TrackingHelper{}).RecordClick(TrackingClick{TrackingId: "tracking-id-0123456789", Url: "https://example.com"})
		return err
	})
}

func (test *TrackingHelperTest) TestActivityNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&TrackingHelper{}).Activity(TrackingActivityOptions{AllowedDomains: []string{"example.com"}, OwnerEmail: "ada@example.com"})
		return err
	})
}
