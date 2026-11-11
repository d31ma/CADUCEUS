// @Test(AdminShellTestSetup, {"partition":"tests"})
type AdminProblemServiceTest struct {
	behaviors *BehaviorContext
}

func (test *AdminProblemServiceTest) TestNoErrorIsNoProblem() {
	Assert.Equals("", (&AdminProblemService{}).Of(nil))
}

func (test *AdminProblemServiceTest) TestForbiddenIsNamed() {
	Assert.Equals("forbidden", (&AdminProblemService{}).Of(&BehaviorError{Code: "HTTP_STATUS", Status: 403}))
}

func (test *AdminProblemServiceTest) TestConflictIsNamed() {
	Assert.Equals("conflict", (&AdminProblemService{}).Of(&BehaviorError{Code: "HTTP_STATUS", Status: 409}))
}

func (test *AdminProblemServiceTest) TestANetworkFailureIsUnavailable() {
	Assert.Equals("unavailable", (&AdminProblemService{}).Of(&BehaviorError{Code: "HTTP_NETWORK"}))
}
