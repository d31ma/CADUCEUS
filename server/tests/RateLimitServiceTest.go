// RateLimitServiceTest calls every method of RateLimitService on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(RateLimitTestSetup)
type RateLimitServiceTest struct {
	behaviors *BehaviorContext
}

func (test *RateLimitServiceTest) TestCheckNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&RateLimitService{}).Check([]string{"login", "ada@example.com"}, 5, 60_000)
		return err
	})
}
