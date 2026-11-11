// @Test(ChannelsTestSetup, {"partition":"tests"})
type ChannelsErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *ChannelsErrorHandlerTest) TestEveryErrorHasItsStableCode() {
	handler := &ChannelsErrorHandler{}
	Assert.Equals(ApiError{Code: "invalid_request", Message: "bad", Field: "name"}, handler.Invalid(&InvalidRequest{Message: "bad", Field: "name"}))
	Assert.Equals("unauthenticated", handler.Unauthenticated(&Unauthenticated{Message: "no"}).Code)
	Assert.Equals("forbidden", handler.Forbidden(&Forbidden{Message: "no"}).Code)
	Assert.Equals("not_found", handler.Missing(&NotFound{Message: "no"}).Code)
	Assert.Equals("conflict", handler.Conflict(&Conflict{Message: "no"}).Code)
	Assert.Equals("duplicate", handler.Duplicate(&Duplicate{Message: "no"}).Code)
	Assert.Equals("unavailable", handler.Unavailable(&Unavailable{Message: "no"}).Code)
}
