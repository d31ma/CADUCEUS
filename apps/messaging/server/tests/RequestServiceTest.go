import "mesail.dev/packages/core"

// @Test(MessagingTestSetup, {"partition":"tests"})
type RequestServiceTest struct {
	behaviors *BehaviorContext
}

func (test *RequestServiceTest) TestAuthorizationReadsTheHeader() {
	world := MessagingTestSetup{}.World()
	Assert.Equals("Bearer abc", (&RequestService{}).Authorization(world.Request("/x", "Bearer abc")))
}

func (test *RequestServiceTest) TestPageWithoutValuesLeavesTheDefaults() {
	world := MessagingTestSetup{}.World()
	page, err := (&RequestService{}).Page(world.Request("/x", ""))
	Assert.IsTrue(err == nil, "an empty page was refused")
	Assert.Equals(core.PageRequest{}, page)
}

func (test *RequestServiceTest) TestPageReadsTheCursorAndTheLimit() {
	world := MessagingTestSetup{}.World()
	page, err := (&RequestService{}).Page(world.Request("/x?cursor=abc&limit=100", ""))
	Assert.IsTrue(err == nil, "a valid page was refused")
	Assert.Equals(core.PageRequest{Cursor: "abc", Limit: 100}, page)
}

func (test *RequestServiceTest) TestPageRefusesALimitAboveOneHundred() {
	world := MessagingTestSetup{}.World()
	_, err := (&RequestService{}).Page(world.Request("/x?limit=101", ""))
	Assert.Equals("limit", world.InvalidField(err))
}

func (test *RequestServiceTest) TestPageRefusesALimitOfZero() {
	world := MessagingTestSetup{}.World()
	_, err := (&RequestService{}).Page(world.Request("/x?limit=0", ""))
	Assert.Equals("limit", world.InvalidField(err))
}

func (test *RequestServiceTest) TestMessageQueryReadsTheThreadRoot() {
	world := MessagingTestSetup{}.World()
	query, err := (&RequestService{}).MessageQuery("cnv_1", world.Request("/x?threadRootId=msg_2&limit=5", ""))
	Assert.IsTrue(err == nil, "a valid query was refused")
	Assert.Equals(MessageQuery{ConversationId: "cnv_1", ThreadRootId: "msg_2", Page: core.PageRequest{Limit: 5}}, query)
}
