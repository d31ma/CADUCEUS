import (
	"cmp"
	"time"
)

// @Service
type ConversationTimeService struct{}

func (service *ConversationTimeService) Time(timestamp string) string {
	moment, valid := service.parse(timestamp)
	return service.onlyWhenValid(valid, moment.Format(service.timeLayout()))
}

func (service *ConversationTimeService) Day(timestamp string, nowMs float64) string {
	moment, valid := service.parse(timestamp)
	now := time.UnixMilli(int64(nowMs)).UTC()
	label := cmp.Or(service.nearDays(now)[service.dateKey(moment)], moment.Format(service.dayLayout(moment, now)))
	return service.onlyWhenValid(valid, label)
}

func (service *ConversationTimeService) parse(timestamp string) (time.Time, bool) {
	moment, failure := time.Parse(time.RFC3339, timestamp)
	return moment.UTC(), failure == nil
}

func (service *ConversationTimeService) nearDays(now time.Time) map[string]string {
	return map[string]string{
		service.dateKey(now):                   "Today",
		service.dateKey(now.AddDate(0, 0, -1)): "Yesterday",
	}
}

func (service *ConversationTimeService) dateKey(moment time.Time) string {
	return moment.Format("2006-01-02")
}

func (service *ConversationTimeService) dayLayout(moment time.Time, now time.Time) string {
	if moment.Year() == now.Year() {
		return "Monday 2 January"
	}
	return "Monday 2 January 2006"
}

func (service *ConversationTimeService) onlyWhenValid(valid bool, text string) string {
	if valid {
		return text
	}
	return ""
}

func (service *ConversationTimeService) timeLayout() string {
	return "15:04"
}
