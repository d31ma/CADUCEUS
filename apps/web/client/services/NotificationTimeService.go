import (
	"math"
	"time"
)

// @Service
type NotificationTimeService struct {
	zone *time.Location
	// @Needs
	clock Clock
}

func (service *NotificationTimeService) Stamp() string {
	return service.universalText(service.now())
}

func (service *NotificationTimeService) Later(minutes int) string {
	return service.universalText(service.now().Add(time.Duration(minutes) * time.Minute))
}

func (service *NotificationTimeService) TomorrowMorning() string {
	tomorrow := service.midnight(service.now()).AddDate(0, 0, 1)
	return service.universalText(tomorrow.Add(service.morningHour() * time.Hour))
}

func (service *NotificationTimeService) ClockText(stamp string) string {
	return service.parse(stamp).In(service.location()).Format(service.clockLayout())
}

func (service *NotificationTimeService) DayText(stamp string) string {
	moment := service.parse(stamp)
	names := map[int]string{0: "Today", 1: "Yesterday"}
	return service.namedOr(names, service.daysBefore(moment), moment.In(service.location()).Format("Monday 2 January"))
}

func (service *NotificationTimeService) EndText(stamp string) string {
	moment := service.parse(stamp)
	prefixes := map[int]string{0: "", -1: "tomorrow "}
	return service.namedOr(prefixes, service.daysBefore(moment), moment.In(service.location()).Format("Monday ")) + service.ClockText(stamp)
}

func (service *NotificationTimeService) Passed(stamp string) bool {
	return service.passed(stamp, service.now())
}

func (service *NotificationTimeService) passed(stamp string, now time.Time) bool {
	return stamp != "" && !now.Before(service.parse(stamp))
}

func (service *NotificationTimeService) namedOr(names map[int]string, days int, fallback string) string {
	name, found := names[days]
	if found {
		return name
	}
	return fallback
}

func (service *NotificationTimeService) daysBefore(moment time.Time) int {
	difference := service.midnight(service.now()).Sub(service.midnight(moment))
	return int(math.Round(difference.Hours() / service.hoursPerDay()))
}

func (service *NotificationTimeService) midnight(moment time.Time) time.Time {
	local := moment.In(service.location())
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, service.location())
}

func (service *NotificationTimeService) location() *time.Location {
	if service.zone == nil {
		return time.Local
	}
	return service.zone
}

func (service *NotificationTimeService) now() time.Time {
	milliseconds, _ := service.clock.NowMs()
	return time.UnixMilli(int64(milliseconds))
}

func (service *NotificationTimeService) parse(stamp string) time.Time {
	moment, _ := time.Parse(time.RFC3339, stamp)
	return moment
}

func (service *NotificationTimeService) universalText(moment time.Time) string {
	return moment.UTC().Format(time.RFC3339)
}

func (service *NotificationTimeService) clockLayout() string {
	return "15:04"
}

func (service *NotificationTimeService) morningHour() time.Duration {
	return 9
}

func (service *NotificationTimeService) hoursPerDay() float64 {
	return 24
}
