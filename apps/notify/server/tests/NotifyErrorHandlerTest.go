import "errors"

// @Test(NotifyTestSetup, {"partition":"tests"})
type NotifyErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *NotifyErrorHandlerTest) handler() *NotifyErrorHandler {
	return &NotifyErrorHandler{}
}

func (test *NotifyErrorHandlerTest) TestUnexpectedErrorIsTheFirstNumber() {
	answer := test.handler().Unexpected(errors.New("disk full at /var/lib"))
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS4500]: Something went wrong.", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestRefusedTokenIsNumbered() {
	answer := test.handler().Refused(Unauthenticated{}.TokenRefused())
	Assert.Equals("unauthenticated", answer.Code)
	Assert.Equals("[MS4501]: Sign in again.", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestInstallWideRouteIsNumbered() {
	answer := test.handler().Missing(NotFound{}.InstallWideRoute())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("[MS4502]: The install-wide route was not found.", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestServiceOnlyIsNumbered() {
	answer := test.handler().Forbidden(NotService{}.ServiceOnly())
	Assert.Equals("forbidden", answer.Code)
	Assert.Equals("[MS4503]: Only another Mesail service may call this route.", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestMissingDomainIsNumbered() {
	answer := test.handler().Missing(NotFound{}.Domain())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("[MS4504]: The domain was not found.", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestBadEnvelopeKeepsTheReason() {
	answer := test.handler().Invalid(InvalidRequest{}.BadEnvelope("the event id is missing"))
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("envelope", answer.Field)
	Assert.Equals("[MS4505]: the event id is missing", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestUnknownEventTypeNamesTheType() {
	answer := test.handler().Invalid(InvalidRequest{}.UnknownEventType("channel.created"))
	Assert.Equals("type", answer.Field)
	Assert.Equals("[MS4506]: notify does not handle events of type channel.created", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestBadTimeIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.BadTime())
	Assert.Equals("time", answer.Field)
	Assert.Equals("[MS4507]: the time is not an RFC 3339 time", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestBadLimitIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.BadLimit("limit must be a whole number from 1 to 100"))
	Assert.Equals("limit", answer.Field)
	Assert.Equals("[MS4508]: limit must be a whole number from 1 to 100", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestBadCursorIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.BadCursor("cursor is not readable"))
	Assert.Equals("cursor", answer.Field)
	Assert.Equals("[MS4509]: cursor is not readable", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestMissingNotificationIsNumbered() {
	answer := test.handler().Missing(NotFound{}.Notification())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("[MS4510]: The notification was not found.", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestMissingMessageIsNumbered() {
	answer := test.handler().Missing(NotFound{}.Message())
	Assert.Equals("[MS4511]: The message was not found.", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestBadSummaryDelayIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.BadSummaryDelay("must be 0 or between 15 and 1440"))
	Assert.Equals("emailSummaryAfterMinutes", answer.Field)
	Assert.Equals("[MS4512]: must be 0 or between 15 and 1440", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestBadMutedListIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.BadMutedList("may hold at most 500 ids"))
	Assert.Equals("mutedConversationIds", answer.Field)
	Assert.Equals("[MS4513]: may hold at most 500 ids", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestBadEndpointIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.BadEndpoint("the endpoint is not a valid URL"))
	Assert.Equals("endpoint", answer.Field)
	Assert.Equals("[MS4514]: the endpoint is not a valid URL", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestMissingKeysIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.MissingKeys())
	Assert.Equals("p256dh", answer.Field)
	Assert.Equals("[MS4515]: the subscription keys p256dh and auth are required", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestPushNotConfiguredIsNumbered() {
	answer := test.handler().PushUnavailable(PushNotConfigured{}.NoKeys())
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("[MS4516]: Web push is not configured on this server.", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestStoreFailureHidesTheOperation() {
	failure := StorageUnavailable{}.Store("insert")
	answer := test.handler().Unavailable(failure)
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("[MS4517]: Notification storage is not available. Try again later.", answer.Message)
	Assert.Equals("notify storage failed: insert", failure.Error())
}

func (test *NotifyErrorHandlerTest) TestOutboxSettingsFailureIsNumbered() {
	answer := test.handler().Unavailable(StorageUnavailable{}.OutboxSettings())
	Assert.Equals("[MS4518]: Notification storage is not available. Try again later.", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestOutboxRelayFailureIsNumbered() {
	answer := test.handler().Unavailable(StorageUnavailable{}.OutboxRelay())
	Assert.Equals("[MS4519]: Notification storage is not available. Try again later.", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestMissingOutboxDocumentIsNumbered() {
	answer := test.handler().Missing(NotFound{}.OutboxDocument())
	Assert.Equals("[MS4520]: The outbox document was not found.", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestMissingOutboxCollectionIsNumbered() {
	answer := test.handler().Missing(NotFound{}.OutboxCollection())
	Assert.Equals("[MS4521]: The outbox collection was not found.", answer.Message)
}

func (test *NotifyErrorHandlerTest) TestMissingDeadLetterIsNumbered() {
	answer := test.handler().Missing(NotFound{}.DeadLetter())
	Assert.Equals("[MS4522]: The dead letter was not found.", answer.Message)
}
