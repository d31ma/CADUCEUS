import (
	"errors"
	"slices"

	"mesail.dev/packages/apierrors"
)

// @Test(ChannelsTestSetup, {"partition":"tests"})
type ChannelsErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *ChannelsErrorHandlerTest) TestEveryErrorHasItsStableCodeAndNumber() {
	handler := &ChannelsErrorHandler{}
	Assert.Equals(ApiError{Code: "invalid_request", Message: "[MS2001]: bad", Field: "name"}, handler.Invalid(&InvalidRequest{Message: "bad", Field: "name", ErrorNumber: 2001}))
	Assert.Equals(ApiError{Code: "unauthenticated", Message: "[MS2002]: no"}, handler.Unauthenticated(&Unauthenticated{Message: "no", ErrorNumber: 2002}))
	Assert.Equals(ApiError{Code: "forbidden", Message: "[MS2003]: no"}, handler.Forbidden(&Forbidden{Message: "no", ErrorNumber: 2003}))
	Assert.Equals(ApiError{Code: "not_found", Message: "[MS2004]: no"}, handler.Missing(&NotFound{Message: "no", ErrorNumber: 2004}))
	Assert.Equals(ApiError{Code: "conflict", Message: "[MS2005]: no"}, handler.Conflict(&Conflict{Message: "no", ErrorNumber: 2005}))
	Assert.Equals(ApiError{Code: "duplicate", Message: "[MS2006]: no"}, handler.Duplicate(&Duplicate{Message: "no", ErrorNumber: 2006}))
	Assert.Equals(ApiError{Code: "unavailable", Message: "[MS2007]: no"}, handler.Unavailable(&Unavailable{Message: "no", ErrorNumber: 2007}))
}

func (test *ChannelsErrorHandlerTest) TestAnUnexpectedErrorAnswersTheUnexpectedNumber() {
	answer := (&ChannelsErrorHandler{}).Unexpected(errors.New("the disk is full at /var/data"))
	Assert.Equals(ApiError{Code: "unavailable", Message: "[MS2000]: Something went wrong."}, answer)
}

func (test *ChannelsErrorHandlerTest) TestEveryNumberIsInsideTheChannelsBlock() {
	block := apierrors.NumberBlock{First: 2000, Last: 2499}
	numbers := []int{2000, 2001, 2002, 2003, 2004, 2005, 2006, 2007, 2008, 2009, 2010, 2011, 2012, 2013, 2014, 2015, 2016, 2017, 2018, 2019, 2020, 2021, 2022, 2023, 2025, 2026, 2027, 2028}
	Assert.Equals(numbers, slices.DeleteFunc(slices.Clone(numbers), test.outside(block)))
}

func (test *ChannelsErrorHandlerTest) outside(block apierrors.NumberBlock) func(int) bool {
	return func(number int) bool { return !block.Holds(number) }
}
