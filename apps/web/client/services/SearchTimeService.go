import (
	"cmp"
	"time"
	_ "time/tzdata"
)

// @Service
type SearchTimeService struct {
	// @Needs
	clock Clock
}

func (service *SearchTimeService) LabelOf(createdAt string) string {
	instant, failure := time.Parse(time.RFC3339Nano, createdAt)
	location := service.location()
	return service.labelUnlessFailed(instant.In(location), service.now().In(location), failure, createdAt)
}

func (service *SearchTimeService) labelUnlessFailed(moment time.Time, now time.Time, failure error, createdAt string) string {
	if failure != nil {
		return createdAt
	}
	return service.localLabel(moment, now)
}

func (service *SearchTimeService) localLabel(moment time.Time, now time.Time) string {
	day := cmp.Or(service.nearDays(now)[service.dateKey(moment)], moment.Format(service.dateLayout(moment, now)))
	return day + moment.Format(service.clockLayout())
}

func (service *SearchTimeService) nearDays(now time.Time) map[string]string {
	return map[string]string{
		service.dateKey(now):                   "Today ",
		service.dateKey(now.AddDate(0, 0, -1)): "Yesterday ",
	}
}

func (service *SearchTimeService) dateLayout(moment time.Time, now time.Time) string {
	if moment.Year() == now.Year() {
		return "2 Jan, "
	}
	return "2 Jan 2006, "
}

func (service *SearchTimeService) dateKey(moment time.Time) string {
	return moment.Format("2006-01-02")
}

func (service *SearchTimeService) location() *time.Location {
	zone, _ := service.clock.TimeZone()
	location, failure := time.LoadLocation(zone)
	if failure != nil {
		return time.UTC
	}
	return location
}

func (service *SearchTimeService) now() time.Time {
	milliseconds, _ := service.clock.NowMs()
	return time.UnixMilli(int64(milliseconds))
}

func (service *SearchTimeService) clockLayout() string {
	return "15:04"
}
