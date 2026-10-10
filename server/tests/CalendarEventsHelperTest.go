// CalendarEventsHelperTest calls every method of CalendarEventsHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(CalendarEventsTestSetup)
type CalendarEventsHelperTest struct {
	behaviors *BehaviorContext
}

func (test *CalendarEventsHelperTest) TestPutNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&CalendarEventsHelper{}).Put(map[string]any{"id": "event-1", "attendees": []any{}})
		return err
	})
}

func (test *CalendarEventsHelperTest) TestListNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&CalendarEventsHelper{}).List("ada@example.com", "2026-01-01T00:00:00.000Z", "")
		return err
	})
}

func (test *CalendarEventsHelperTest) TestFindByIdNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&CalendarEventsHelper{}).FindById("event-1")
		return err
	})
}

func (test *CalendarEventsHelperTest) TestFindByIcsUidNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&CalendarEventsHelper{}).FindByIcsUid("ada@example.com", "uid-1")
		return err
	})
}

func (test *CalendarEventsHelperTest) TestFindByIdempotencyKeyNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&CalendarEventsHelper{}).FindByIdempotencyKey("ada@example.com", "key-1")
		return err
	})
}

func (test *CalendarEventsHelperTest) TestFindExceptionNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&CalendarEventsHelper{}).FindException("event-1", "2026-01-02")
		return err
	})
}

func (test *CalendarEventsHelperTest) TestDeleteSeriesNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&CalendarEventsHelper{}).DeleteSeries("event-1", "ada@example.com")
		return err
	})
}

func (test *CalendarEventsHelperTest) TestDeleteNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&CalendarEventsHelper{}).Delete("doc-1")
		return err
	})
}
