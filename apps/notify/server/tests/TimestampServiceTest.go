import "errors"

// @Test(NotifyTestSetup, {"partition":"tests"})
type TimestampServiceTest struct {
	behaviors *BehaviorContext
}

func (test *TimestampServiceTest) TestNowHasMillisecondsAndZ() {
	now := (&TimestampService{}).Now()
	Assert.Equals(24, len(now))
	Assert.Equals("Z", now[23:])
}

func (test *TimestampServiceTest) TestParseReadsAnRfc3339Time() {
	moment, err := (&TimestampService{}).Parse("2026-11-10T22:00:00.000Z")
	Assert.IsTrue(err == nil, "parse failed")
	Assert.Equals(2026, moment.Year())
}

func (test *TimestampServiceTest) TestParseRefusesText() {
	_, err := (&TimestampService{}).Parse("yesterday")
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid))
}

func (test *TimestampServiceTest) TestMonthIsYearAndMonth() {
	month, err := (&TimestampService{}).Month("2026-11-10T22:00:00.000Z")
	Assert.IsTrue(err == nil, "month failed")
	Assert.Equals("202611", month)
}
