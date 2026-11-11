// @Test(DirectoryTestSetup, {"partition":"tests"})
type DirectoryErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *DirectoryErrorHandlerTest) TestInvalidRequestNamesTheField() {
	answer := (&DirectoryErrorHandler{}).Invalid(&InvalidRequest{Field: "name", Message: "bad"})
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("name", answer.Field)
}

func (test *DirectoryErrorHandlerTest) TestUnauthenticatedHasItsCode() {
	Assert.Equals("unauthenticated", (&DirectoryErrorHandler{}).Unauthenticated(&Unauthenticated{}).Code)
}

func (test *DirectoryErrorHandlerTest) TestForbiddenHasItsCode() {
	Assert.Equals("forbidden", (&DirectoryErrorHandler{}).Forbidden(&Forbidden{}).Code)
}

func (test *DirectoryErrorHandlerTest) TestALastAdminIsAConflict() {
	answer := (&DirectoryErrorHandler{}).LastAdmin(&LastAdmin{Message: "a domain must keep at least one admin"})
	Assert.Equals("last_admin", answer.Code)
	Assert.Equals("a domain must keep at least one admin", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestNotFoundHasItsCode() {
	Assert.Equals("not_found", (&DirectoryErrorHandler{}).Missing(&NotFound{}).Code)
}

func (test *DirectoryErrorHandlerTest) TestDomainNotHostedHasItsOwnCode() {
	Assert.Equals("domain_not_hosted", (&DirectoryErrorHandler{}).NotHosted(&DomainNotHosted{}).Code)
}

func (test *DirectoryErrorHandlerTest) TestDuplicateHasItsCode() {
	Assert.Equals("duplicate", (&DirectoryErrorHandler{}).Duplicate(&Duplicate{}).Code)
}

func (test *DirectoryErrorHandlerTest) TestUnavailableHidesTheCause() {
	answer := (&DirectoryErrorHandler{}).Unavailable(&Unavailable{Message: "the directory store is not available", Cause: &NotFound{Message: "secret detail"}})
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("the directory store is not available", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestAStoreFailureIsUnavailable() {
	answer := (&DirectoryErrorHandler{}).Store(YonResourceError{})
	Assert.Equals("unavailable", answer.Code)
}
