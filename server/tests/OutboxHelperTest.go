// OutboxHelperTest calls the outbox helper without a FYLO bridge: every
// operation reaches the boundary and answers its refusal. The routes that
// use the helper test its stored documents over HTTP.
// @Test(OutboxTestSetup)
type OutboxHelperTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxHelperTest) helper() *OutboxHelper {
	return &OutboxHelper{fyloHelper: &FyloHelper{}}
}

func (test *OutboxHelperTest) TestPutNeedsTheBoundary() {
	_, err := test.helper().Put(map[string]any{"id": "undo-1"})
	Assert.IsTrue(err != nil)
}

func (test *OutboxHelperTest) TestFindByIdNeedsTheBoundary() {
	_, _, err := test.helper().FindById("undo-1")
	Assert.IsTrue(err != nil)
}

func (test *OutboxHelperTest) TestDeleteNeedsTheBoundary() {
	Assert.IsTrue(test.helper().Delete("doc-1") != nil)
}

func (test *OutboxHelperTest) TestFindExpiredNeedsTheBoundary() {
	_, err := test.helper().FindExpired()
	Assert.IsTrue(err != nil)
}

func (test *OutboxHelperTest) TestFindDueNeedsTheBoundary() {
	_, err := test.helper().FindDue(EpochTime{}.Now())
	Assert.IsTrue(err != nil)
}

func (test *OutboxHelperTest) TestListAllNeedsTheBoundary() {
	_, err := test.helper().ListAll()
	Assert.IsTrue(err != nil)
}

func (test *OutboxHelperTest) TestUpdateNeedsTheBoundary() {
	Assert.IsTrue(test.helper().Update("doc-1", map[string]any{"attempts": 1}) != nil)
}
