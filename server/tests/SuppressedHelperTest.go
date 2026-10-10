// SuppressedHelperTest calls every method of SuppressedHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(SuppressedTestSetup)
type SuppressedHelperTest struct {
	behaviors *BehaviorContext
}

func (test *SuppressedHelperTest) TestListNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&SuppressedHelper{}).List()
		return err
	})
}

func (test *SuppressedHelperTest) TestAsSetNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&SuppressedHelper{}).AsSet()
		return err
	})
}

func (test *SuppressedHelperTest) TestSuppressNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&SuppressedHelper{}).Suppress("bounce@example.com", "hard-bounce")
		return err
	})
}

func (test *SuppressedHelperTest) TestDeleteNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&SuppressedHelper{}).Delete("bounce@example.com")
		return err
	})
}
