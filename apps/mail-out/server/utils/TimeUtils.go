import "time"

type TimeUtils struct{}

func (TimeUtils) Format(moment time.Time) string {
	return moment.UTC().Format(time.RFC3339)
}

func (TimeUtils) Parse(text string) time.Time {
	moment, _ := time.Parse(time.RFC3339, text)
	return moment
}
