// PushSubscriptionsHelperTest calls every method of PushSubscriptionsHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(PushSubscriptionsTestSetup)
type PushSubscriptionsHelperTest struct {
	behaviors *BehaviorContext
}

func (test *PushSubscriptionsHelperTest) TestListForUserNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&PushSubscriptionsHelper{}).ListForUser("ada@example.com")
		return err
	})
}

func (test *PushSubscriptionsHelperTest) TestListForAddressNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&PushSubscriptionsHelper{}).ListForAddress("Ada@example.com")
		return err
	})
}

func (test *PushSubscriptionsHelperTest) TestFindByIdNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&PushSubscriptionsHelper{}).FindById("subscription-1")
		return err
	})
}

func (test *PushSubscriptionsHelperTest) TestUpsertNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&PushSubscriptionsHelper{}).Upsert(map[string]any{"endpoint": "https://push.example.com/1", "userEmail": "Ada@example.com"})
		return err
	})
}

func (test *PushSubscriptionsHelperTest) TestDeleteDocumentNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&PushSubscriptionsHelper{}).DeleteDocument("doc-1")
		return err
	})
}
