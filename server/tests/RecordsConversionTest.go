import "encoding/json"

// RecordsConversionTest checks the record conversions of the ported helpers
// against what the JavaScript records answered for the same input. Objects
// inside stored JSON text have their keys sorted, where JavaScript kept the
// order they were written in; the values are the same.
// @Test(RecordsTestSetup)
type RecordsConversionTest struct {
	behaviors *BehaviorContext
}

func (test *RecordsConversionTest) object(text string) map[string]any {
	object := map[string]any{}
	_ = json.Unmarshal([]byte(text), &object)
	return object
}

func (test *RecordsConversionTest) contact() map[string]any {
	return map[string]any{"docId": "d1", "id": "c1", "ownerEmail": "ada@example.com", "emails": []any{map[string]any{"value": "B@x.com", "type": "work"}},
		"phones": nil, "urls": "x", "groupIds": []any{"g1"}, "name": "Bob <b>"}
}

func (test *RecordsConversionTest) TestSerializesAContact() {
	Assert.Equals(`{"addressesJson":"[]","emailsJson":"[{\"type\":\"work\",\"value\":\"B@x.com\"}]","groupIdsJson":"[\"g1\"]","id":"c1","name":"Bob <b>","nicknamesJson":"[]","ownerEmail":"ada@example.com","phonesJson":"[]","urlsJson":"\"x\""}`, StoredRecords{}.JsonText(ContactsRecords{}.Serialized(test.contact())))
}

func (test *RecordsConversionTest) TestHydratesAContact() {
	stored := ContactsRecords{}.Serialized(test.contact())
	stored["addressesJson"] = "{bad"
	Assert.Equals(`{"addresses":[],"emails":[{"type":"work","value":"B@x.com"}],"groupIds":["g1"],"id":"c1","name":"Bob <b>","nicknames":[],"ownerEmail":"ada@example.com","phones":[],"urls":"x"}`, StoredRecords{}.JsonText(ContactsRecords{}.Hydrated(stored)))
}

func (test *RecordsConversionTest) TestHydratesCalendarAttendees() {
	event := CalendarEventsRecords{}.Hydrated(map[string]any{"id": "e1", "attendeesJson": `[{"email":"a@x.com"}]`})
	Assert.Equals(`{"attendees":[{"email":"a@x.com"}],"id":"e1"}`, StoredRecords{}.JsonText(event))
}

func (test *RecordsConversionTest) TestReadsUnreadableAttendeesAsNone() {
	event := CalendarEventsRecords{}.Hydrated(map[string]any{"id": "e2", "attendeesJson": "nope"})
	Assert.Equals(`{"attendees":[],"id":"e2"}`, StoredRecords{}.JsonText(event))
}

func (test *RecordsConversionTest) TestStoresCalendarAttendeesAsText() {
	stored := CalendarEventsRecords{}.Stored(map[string]any{"docId": "d1", "id": "e1", "attendees": []any{map[string]any{"email": "a@x.com"}}})
	Assert.Equals(`{"attendeesJson":"[{\"email\":\"a@x.com\"}]","id":"e1"}`, StoredRecords{}.JsonText(stored))
}

func (test *RecordsConversionTest) TestStoresDomainRoutesAsText() {
	routes := []any{map[string]any{"id": "r1", "match": "<a>&", "action": map[string]any{"type": "drop"}}}
	stored := DomainRecords{}.Encoded(map[string]any{"domain": "example.com", "routes": routes})
	Assert.Equals(`{"domain":"example.com","routes":"[{\"action\":{\"type\":\"drop\"},\"id\":\"r1\",\"match\":\"<a>&\"}]"}`, StoredRecords{}.JsonText(stored))
}

func (test *RecordsConversionTest) TestNamesAPushSubscriptionByItsEndpoint() {
	Assert.Equals("8dd0d552718c5ad0827548b1ea199b5f640e3d6ec8856da362e36941c9566ddb", PushSubscriptionsRecords{}.SubscriptionId("https://push.example.com/1"))
}

func (test *RecordsConversionTest) TestKeepsTextParameterKeys() {
	Assert.Equals(`["a","b"]`, StoredRecords{}.JsonText(McpAuditEventsRecords{}.ParameterKeys(`["a", 2, "b"]`)))
}

func (test *RecordsConversionTest) TestFormatsARateLimitResult() {
	Assert.Equals(`{"allowed":false,"limit":5,"remaining":0,"resetAt":1000,"retryAfterSeconds":60}`, StoredRecords{}.JsonText(RateLimitRules{}.Result(6, 5, 1000, 940)))
}

func (test *RecordsConversionTest) TestHashesARateLimitKey() {
	Assert.Equals("2e98898f9a89ba5194122edbcc95d2d239c8823b30b5c6f4f1509409b13e8776", RateLimitRules{}.HashKey([]string{"login", "ada@example.com"}))
}

func (test *RecordsConversionTest) mail() []map[string]any {
	var mail []map[string]any
	_ = json.Unmarshal([]byte(`[{"clickCount":1,"clicksJson":"[{\"id\":\"k1\",\"url\":\"https://example.com\",\"title\":\"Ex\",\"clickedAt\":\"2026-03-03T09:30:00.000Z\"}]","domain":"example.com","folder":"sent","id":"m1","linkTracking":true,"openCount":"2","openTracking":true,"opensJson":"[{\"id\":\"o1\",\"recipient\":\"BOB@x.com\",\"openedAt\":\"2026-03-02T11:00:00Z\"},{\"openedAt\":\"nope\"}]","receivedAt":"2026-03-02T10:00:00.000Z","sender":"ada@example.com","subject":"Hello\u0001there","threadId":"t1","to":["bob@x.com"]},{"domain":"Example.com","folder":"SENT","id":"m2","openTracking":true,"openedAt":"2026-03-01T09:00:00.000Z","ownerEmail":"ada@example.com","receivedAt":"2026-03-01T08:00:00.000Z","recipient":"carol@x.com, dan@x.com","subject":"Hello\u0001there","threadId":"t2"},{"domain":"example.com","folder":"inbox","id":"m3","receivedAt":"2026-03-02T12:00:00.000Z","sender":"bob@x.com","subject":"Re: Hello","threadId":"t1","to":["ada@example.com"]},{"domain":"other.org","folder":"sent","id":"m4","receivedAt":"2026-03-02T12:00:00.000Z","sender":"ada@example.com","subject":"Elsewhere"},{"domain":"example.com","folder":"sent","id":"m5","receivedAt":"2026-03-02T12:00:00.000Z","sender":"eve@example.com","subject":"Not mine"},{"domain":"example.com","folder":"sent","id":"m6","receivedAt":"2026-02-20T12:00:00.000Z","sender":"ada@example.com","subject":""}]`), &mail)
	return mail
}

func (test *RecordsConversionTest) number(value float64) *float64 {
	return &value
}

func (test *RecordsConversionTest) TestBuildsAWindowedActivityFeed() {
	options := TrackingActivityOptions{AllowedDomains: []string{"example.com"}, OwnerEmail: "Ada@Example.com", Type: "all",
		From: test.number(1772323200000), To: test.number(1772582400000),
		Since: test.number(1772452800000), Limit: test.number(2)}
	activity := TrackingActivity{Options: options, Now: 0}.Built(test.mail())
	Assert.Equals(`{"events":[{"id":"k1","messageId":"m1","recipient":"bob@x.com","subject":"Hello there","threadId":"t1","timestamp":"2026-03-03T09:30:00.000Z","title":"Ex","type":"click","url":"https://example.com"},{"id":"o1","messageId":"m1","recipient":"bob@x.com","subject":"Hello there","threadId":"t1","timestamp":"2026-03-02T11:00:00.000Z","type":"open"}],"generatedAt":"2026-03-04T00:00:00.000Z","metrics":{"bySubject":[{"clicked":1,"opened":2,"replied":1,"sent":2,"subject":"Hello there"}],"clickRate":100,"clicked":1,"daily":[{"clicks":0,"date":"2026-03-01","opens":1,"received":0,"sent":1},{"clicks":0,"date":"2026-03-02","opens":1,"received":1,"sent":1},{"clicks":1,"date":"2026-03-03","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-03-04","opens":0,"received":0,"sent":0}],"openRate":100,"opened":2,"received":1,"replied":1,"replyRate":50,"sent":2,"tracked":2,"trackingRate":100},"unseenCount":1}`, StoredRecords{}.JsonText(activity))
}

func (test *RecordsConversionTest) TestBuildsTheWholeActivityFeed() {
	options := TrackingActivityOptions{AllowedDomains: []string{"example.com"}, OwnerEmail: "ada@example.com", To: test.number(1772582400000)}
	activity := TrackingActivity{Options: options, Now: 0}.Built(test.mail())
	Assert.Equals(`{"events":[{"id":"k1","messageId":"m1","recipient":"bob@x.com","subject":"Hello there","threadId":"t1","timestamp":"2026-03-03T09:30:00.000Z","title":"Ex","type":"click","url":"https://example.com"},{"id":"o1","messageId":"m1","recipient":"bob@x.com","subject":"Hello there","threadId":"t1","timestamp":"2026-03-02T11:00:00.000Z","type":"open"},{"id":"open-m2-2026-03-01T09:00:00.000Z","messageId":"m2","recipient":"carol@x.com","subject":"Hello there","threadId":"t2","timestamp":"2026-03-01T09:00:00.000Z","type":"open"}],"generatedAt":"2026-03-04T00:00:00.000Z","metrics":{"bySubject":[{"clicked":1,"opened":2,"replied":1,"sent":2,"subject":"Hello there"},{"clicked":0,"opened":0,"replied":0,"sent":1,"subject":"(No subject)"}],"clickRate":100,"clicked":1,"daily":[{"clicks":0,"date":"2026-02-03","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-04","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-05","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-06","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-07","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-08","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-09","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-10","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-11","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-12","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-13","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-14","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-15","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-16","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-17","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-18","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-19","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-20","opens":0,"received":0,"sent":1},{"clicks":0,"date":"2026-02-21","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-22","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-23","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-24","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-25","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-26","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-27","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-02-28","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-03-01","opens":1,"received":0,"sent":1},{"clicks":0,"date":"2026-03-02","opens":1,"received":1,"sent":1},{"clicks":1,"date":"2026-03-03","opens":0,"received":0,"sent":0},{"clicks":0,"date":"2026-03-04","opens":0,"received":0,"sent":0}],"openRate":100,"opened":2,"received":1,"replied":1,"replyRate":33.3,"sent":3,"tracked":2,"trackingRate":66.7},"unseenCount":0}`, StoredRecords{}.JsonText(activity))
}
