// DomainMigrationsHelperTest calls every method of DomainMigrationsHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(DomainMigrationsTestSetup)
type DomainMigrationsHelperTest struct {
	behaviors *BehaviorContext
}

func (test *DomainMigrationsHelperTest) TestListNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&DomainMigrationsHelper{}).List()
		return err
	})
}

func (test *DomainMigrationsHelperTest) TestFindNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&DomainMigrationsHelper{}).Find("old.example.com", "new.example.com")
		return err
	})
}

func (test *DomainMigrationsHelperTest) TestPutNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&DomainMigrationsHelper{}).Put(map[string]any{"fromDomain": "old.example.com", "toDomain": "new.example.com"})
		return err
	})
}
