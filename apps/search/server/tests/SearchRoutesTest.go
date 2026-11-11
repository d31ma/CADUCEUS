import (
	"errors"
	"net/http"
	"net/url"

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
	_, err := (&SearchController{dataSource: DataSource{}}).GetSearch(test.request(""))
	Assert.IsTrue(errors.As(err, new(*Unauthenticated)), "expected unauthenticated")
}

func (test *SearchRoutesTest) TestSearchRouteRefusesAServiceToken() {
	world := SearchTestSetup{}.World()
	token := world.SignedIn(world.Domain, "messaging", "mesail-service", "mesail-service")
	_, err := (&SearchController{dataSource: DataSource{}}).GetSearch(test.request(token))
	Assert.IsTrue(errors.As(err, new(*Forbidden)), "expected forbidden")
}

func (test *SearchRoutesTest) TestSearchRouteFailsClosedWhenMembershipCannotBeChecked() {
	world := SearchTestSetup{}.World()
	token := world.SignedIn(world.Domain, "ana", "mesail-member", "mesail-identity")
	_, err := (&SearchController{dataSource: DataSource{}}).GetSearch(test.request(token))
	Assert.IsTrue(errors.As(err, new(*Unavailable)), "expected unavailable")
}

func (test *SearchRoutesTest) TestEventRouteRefusesACallerWithoutAServiceToken() {
	world := SearchTestSetup{}.World()
	envelope := world.Envelope(world.Domain, "message.created", world.Message("cnv_one", "hi", "2026-11-10T10:00:00Z"))
	_, err := (&EventsController{dataSource: DataSource{}}).PostEvent("", test.envelope(envelope))
	Assert.IsTrue(errors.As(err, new(*Forbidden)), "the route accepted a caller without a token")
}

func (test *SearchRoutesTest) TestEventRouteIndexesAMessageFromAServiceToken() {
	world := SearchTestSetup{}.World()
	token := world.SignedIn(world.Domain, "messaging", "mesail-service", "mesail-service")
	message := world.Message("cnv_one", "posted through the route", "2026-11-10T10:00:00Z")
	receipt, err := (&EventsController{dataSource: DataSource{}}).PostEvent(token, test.envelope(world.Envelope(world.Domain, "message.created", message)))
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

func (test *SearchRoutesTest) TestErrorHandlerUsesTheContractCodes() {
	handler := &SearchErrorHandler{}
	Assert.Equals(ApiError{Code: "invalid_request", Message: "bad", Field: "q"}, handler.Invalid(&InvalidRequest{Message: "bad", Field: "q"}))
	Assert.Equals("unauthenticated", handler.Unauthenticated(&Unauthenticated{}).Code)
	Assert.Equals("forbidden", handler.Forbidden(&Forbidden{Message: "no"}).Code)
	Assert.Equals("unavailable", handler.Unavailable(&Unavailable{Message: "down"}).Code)
}

func (test *SearchRoutesTest) TestEventRouteRefusesABadEnvelope() {
	world := SearchTestSetup{}.World()
	token := world.SignedIn(world.Domain, "messaging", "mesail-service", "mesail-service")
	envelope := test.envelope(world.Envelope(world.Domain, "message.created", world.Message("cnv_one", "hi", "2026-11-10T10:00:00Z")))
	envelope.EventId = ""
	_, err := (&EventsController{dataSource: DataSource{}}).PostEvent(token, envelope)
	Assert.IsTrue(errors.As(err, new(*InvalidRequest)), "expected invalid request")
}

func (test *SearchRoutesTest) TestSearchRouteRefusesABadLimit() {
	world := SearchTestSetup{}.World()
	token := world.SignedIn(world.Domain, "ana", "mesail-member", "mesail-identity")
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/search?q=hello&limit=0", nil)
	request.Header.Set("Authorization", token)
	_, err := (&SearchController{dataSource: DataSource{}}).GetSearch(request)
	Assert.IsTrue(errors.As(err, new(*InvalidRequest)), "expected invalid request")
}

func (test *SearchRoutesTest) TestEventRouteAppliesAGoodEnvelope() {
	world := SearchTestSetup{}.World()
	token := world.SignedIn(world.Domain, "messaging", "mesail-service", "mesail-service")
	envelope := test.envelope(world.Envelope(world.Domain, "message.created", world.Message("cnv_one", "hi", "2026-11-10T10:00:00Z")))
	_, err := (&EventsController{dataSource: DataSource{}}).PostEvent(token, envelope)
	Assert.Equals(nil, err)
	Assert.Equals("bad", (&InvalidRequest{Message: "bad"}).Error())
	Assert.Equals("no", (&Forbidden{Message: "no"}).Error())
	Assert.Equals("no", (&Unauthenticated{Message: "no"}).Error())
	Assert.Equals("down", (&Unavailable{Message: "down"}).Error())
}

func (test *SearchRoutesTest) TestAnUnavailableIndexNamesItsCause() {
	failure := search.Unavailable{Message: "the search index is not available", Cause: errors.New("Yon managed resource operation failed.")}
	Assert.Equals("the search index is not available: Yon managed resource operation failed.", failure.Error())
	Assert.Equals("the search index is not available", search.Unavailable{Message: "the search index is not available"}.Error())
}
