import "time"

// @Service
type TimestampService struct{}

func (timestamps *TimestampService) Now() string {
	return timestamps.text(time.Now())
}

func (timestamps *TimestampService) text(moment time.Time) string {
	return moment.UTC().Format(timestamps.layout())
}

func (timestamps *TimestampService) Parse(text string) (time.Time, error) {
	moment, failure := time.Parse(time.RFC3339, text)
	return moment, timestamps.invalid(failure)
}

func (timestamps *TimestampService) Month(text string) (string, error) {
	moment, failure := timestamps.Parse(text)
	return moment.UTC().Format(timestamps.monthLayout()), failure
}

func (timestamps *TimestampService) layout() string {
	return "2006-01-02T15:04:05.000Z"
}

func (timestamps *TimestampService) monthLayout() string {
	return "200601"
}

func (timestamps *TimestampService) invalid(failure error) error {
	if failure != nil {
		return &InvalidRequest{Field: "time", Reason: "the time is not an RFC 3339 time"}
	}
	return nil
}
