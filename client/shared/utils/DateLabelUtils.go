import "time"

// DateLabelUtils formats stored timestamps the way the JavaScript client's
// Intl.DateTimeFormat did for English: "Oct 10, 2026" and "Oct 10, 2026,
// 3:04 PM", in the browser's time zone offset.
type DateLabelUtils struct{}

func (utils DateLabelUtils) Date(value string, offsetMinutes float64) string {
	return utils.format(value, offsetMinutes, "Jan 2, 2006")
}

func (utils DateLabelUtils) DateTime(value string, offsetMinutes float64) string {
	return utils.format(value, offsetMinutes, "Jan 2, 2006, 3:04 PM")
}

func (DateLabelUtils) format(value string, offsetMinutes float64, layout string) string {
	moment, failure := time.Parse(time.RFC3339Nano, value)
	if failure != nil {
		return ""
	}
	return moment.In(time.FixedZone("", int(offsetMinutes*60))).Format(layout)
}
