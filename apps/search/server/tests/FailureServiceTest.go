import (
	"errors"

	"mesail.dev/packages/search"
)

// @Test(HealthTestSetup, {"partition":"tests"})
type FailureServiceTest struct {
	behaviors *BehaviorContext
}

func (test *FailureServiceTest) TestNoFailureStaysNil() {
	Assert.Equals(nil, (&FailureService{}).Translated(nil))
}

func (test *FailureServiceTest) TestAnInvalidRequestKeepsItsCaseNumber() {
	answer := (&FailureService{}).Translated(search.InvalidRequest{}.NotADate("from"))
	Assert.Equals(&InvalidRequest{Message: "from must be a date or an RFC 3339 time", Field: "from", ErrorNumber: 4002}, answer)
}

func (test *FailureServiceTest) TestAForbiddenCallerKeepsItsCaseNumber() {
	answer := (&FailureService{}).Translated(search.Forbidden{}.MembersOnly())
	Assert.Equals(&Forbidden{Message: "only members of a domain search its messages", ErrorNumber: 4013}, answer)
}

func (test *FailureServiceTest) TestOtherFailuresKeepTheirTypes() {
	Assert.Equals(&Unauthenticated{Message: "the access token was refused"}, (&FailureService{}).Translated(search.Unauthenticated{}))
	Assert.Equals(&Unavailable{Message: "boom"}, (&FailureService{}).Translated(errors.New("boom")))
}
