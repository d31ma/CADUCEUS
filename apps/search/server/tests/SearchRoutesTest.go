import (
	"errors"
	"net/http"
	"net/url"

	"mesail.dev/packages/apierrors"
	"mesail.dev/packages/events"
	"mesail.dev/packages/search"
)

// @Test(SearchTestSetup, {"partition":"tests"})
type SearchRoutesTest struct {
	behaviors *BehaviorContext
}

func (test *SearchRoutesTest) request(token string) *http.Request {
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/search?q=hello", nil)
	request.Header.Set("Authorization", token)
	return request
}

func (test *SearchRoutesTest) TestSearchRouteRefusesARequestWithoutAToken() {
	_, err := SearchTestSetup{}.SearchController().GetSearch(test.request(""))
	Assert.IsTrue(errors.As(err, new(*Unauthenticated)), "expected unauthenticated")
}

func (test *SearchRoutesTest) TestSearchRouteRefusesAServiceToken() {
	world := SearchTestSetup{}.World()
	token := world.SignedIn(world.Domain, "messaging", "mesail-service", "mesail-service")
	_, err := SearchTestSetup{}.SearchController().GetSearch(test.request(token))
	forbidden := &Forbidden{}
	Assert.IsTrue(errors.As(err, &forbidden), "expected forbidden")
	Assert.Equals(4013, forbidden.Number())
}

func (test *SearchRoutesTest) TestSearchRouteFailsClosedWhenMembershipCannotBeChecked() {
	world := SearchTestSetup{}.World()
	token := world.SignedIn(world.Domain, "ana", "mesail-member", "mesail-web")
	_, err := SearchTestSetup{}.SearchController().GetSearch(test.request(token))
	Assert.IsTrue(errors.As(err, new(*Unavailable)), "expected unavailable")
}

func (test *SearchRoutesTest) TestEventRouteRefusesACallerWithoutAServiceToken() {
	world := SearchTestSetup{}.World()
	envelope := world.Envelope(world.Domain, "message.created", world.Message("cnv_one", "hi", "2026-11-10T10:00:00Z"))
	_, err := SearchTestSetup{}.EventsController().PostEvent("", test.envelope(envelope))
	forbidden := &Forbidden{}
	Assert.IsTrue(errors.As(err, &forbidden), "the route accepted a caller without a token")
	Assert.Equals(12, forbidden.Number())
}

func (test *SearchRoutesTest) TestEventRouteIndexesAMessageFromAServiceToken() {
	world := SearchTestSetup{}.World()
	token := world.SignedIn(world.Domain, "messaging", "mesail-service", "mesail-service")
	message := world.Message("cnv_one", "posted through the route", "2026-11-10T10:00:00Z")
	receipt, err := SearchTestSetup{}.EventsController().PostEvent(token, test.envelope(world.Envelope(world.Domain, "message.created", message)))
	ana := world.Member(world.Domain, "ana", "cnv_one")
	Assert.Equals(nil, err)
	Assert.Equals("applied", receipt.Status)
	Assert.Equals([]string{message.MessageId}, world.Ids(world.Search(ana, url.Values{"q": {"route"}})))
}

func (test *SearchRoutesTest) envelope(envelope events.Envelope) EventEnvelope {
	model := EventEnvelope{}
	search.Json{}.Copy(envelope, &model)
	return model
}

func (test *SearchRoutesTest) TestErrorHandlerUsesTheContractCodesAndNumbers() {
	handler := &SearchErrorHandler{}
	Assert.Equals(ApiError{Code: "invalid_request", Message: "[MS4005]: bad", Field: "limit"}, handler.Invalid(&InvalidRequest{Message: "bad", Field: "limit", ErrorNumber: 4005}))
	Assert.Equals(ApiError{Code: "unauthenticated", Message: "[MS4012]: Sign in again."}, handler.Unauthenticated(&Unauthenticated{}))
	Assert.Equals(ApiError{Code: "forbidden", Message: "[MS4013]: no"}, handler.Forbidden(&Forbidden{Message: "no", ErrorNumber: 4013}))
	Assert.Equals(ApiError{Code: "unavailable", Message: "[MS4015]: Search is not available. Try again later."}, handler.Unavailable(&Unavailable{Message: "down"}))
	Assert.Equals(ApiError{Code: "unavailable", Message: "[MS4000]: Something went wrong."}, handler.Unexpected(errors.New("disk on fire")))
}

func (test *SearchRoutesTest) TestEveryNumberIsUsedOnceInsideTheSearchBlock() {
	numbers := []int{
		(&UnexpectedFailure{}).Number(),
		search.InvalidRequest{}.QueryTooLong().ErrorNumber, search.InvalidRequest{}.NotADate("from").ErrorNumber,
		search.InvalidRequest{}.NotTrueOrFalse("isEmail").ErrorNumber, search.InvalidRequest{}.BadCursor("").ErrorNumber,
		search.InvalidRequest{}.BadLimit().ErrorNumber, search.InvalidRequest{}.NotATime("createdAt").ErrorNumber,
		search.InvalidRequest{}.MissingIds().ErrorNumber, search.InvalidRequest{}.PayloadMismatch().ErrorNumber,
		search.InvalidRequest{}.NotWritable().ErrorNumber, search.InvalidRequest{}.MissingFields().ErrorNumber,
		(&Unauthenticated{}).Number(), search.Forbidden{}.MembersOnly().ErrorNumber, (&Unavailable{}).Number(),
	}
	block := apierrors.NumberBlock{First: 4000, Last: 4499}
	Assert.Equals([]int{}, SearchTestSetup{}.OutsideBlock(block, numbers))
	Assert.Equals(len(numbers), SearchTestSetup{}.DistinctCount(numbers))
}

func (test *SearchRoutesTest) TestEventRouteRefusesABadEnvelope() {
	world := SearchTestSetup{}.World()
	token := world.SignedIn(world.Domain, "messaging", "mesail-service", "mesail-service")
	envelope := test.envelope(world.Envelope(world.Domain, "message.created", world.Message("cnv_one", "hi", "2026-11-10T10:00:00Z")))
	envelope.EventId = ""
	_, err := SearchTestSetup{}.EventsController().PostEvent(token, envelope)
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid), "expected invalid request")
	Assert.Equals(1, invalid.Number())
}

func (test *SearchRoutesTest) TestSearchRouteRefusesABadLimit() {
	world := SearchTestSetup{}.World()
	token := world.SignedIn(world.Domain, "ana", "mesail-member", "mesail-web")
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/search?q=hello&limit=0", nil)
	request.Header.Set("Authorization", token)
	_, err := SearchTestSetup{}.SearchController().GetSearch(request)
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid), "expected invalid request")
	Assert.Equals(4005, invalid.Number())
}

func (test *SearchRoutesTest) TestEventRouteAppliesAGoodEnvelope() {
	world := SearchTestSetup{}.World()
	token := world.SignedIn(world.Domain, "messaging", "mesail-service", "mesail-service")
	envelope := test.envelope(world.Envelope(world.Domain, "message.created", world.Message("cnv_one", "hi", "2026-11-10T10:00:00Z")))
	_, err := SearchTestSetup{}.EventsController().PostEvent(token, envelope)
	Assert.Equals(nil, err)
	Assert.Equals("bad", (&InvalidRequest{Message: "bad"}).Error())
	Assert.Equals("no", (&Forbidden{Message: "no"}).Error())
	Assert.Equals("no", (&Unauthenticated{Message: "no"}).Error())
	Assert.Equals("down", (&Unavailable{Message: "down"}).Error())
	Assert.Equals("an unexpected error", (&UnexpectedFailure{}).Error())
}

func (test *SearchRoutesTest) TestAnUnavailableIndexNamesItsCause() {
	failure := search.Unavailable{Message: "the search index is not available", Cause: errors.New("Yon managed resource operation failed.")}
	Assert.Equals("the search index is not available: Yon managed resource operation failed.", failure.Error())
	Assert.Equals("the search index is not available", search.Unavailable{Message: "the search index is not available"}.Error())
}
