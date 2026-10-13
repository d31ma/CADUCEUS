import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// RecurrenceRule expands one RFC 5545 RRULE from a start, as the rrule
// library the JavaScript server used did (a port of python-dateutil). Times
// are milliseconds; the rule works on floating times, so the caller converts
// to and from a time zone.
type RecurrenceRule struct {
	Frequency    int
	Interval     int
	Count        int
	Until        int64
	HasUntil     bool
	WeekStart    int
	BySetPos     []int
	ByMonth      []int
	ByMonthDay   []int
	ByNegative   []int
	ByYearDay    []int
	ByWeekNo     []int
	ByWeekday    []int
	ByNthWeekday [][2]int
	Start        int64
}

// Parsed answers the rule of RRULE text (FREQ=WEEKLY;COUNT=3 ...) that
// starts at a floating time.
func (RecurrenceRule) Parsed(text string, start int64) RecurrenceRule {
	rule := RecurrenceRule{Interval: 1, Start: start}
	weekdays := []any{}
	hasWeekdays := false
	for _, part := range strings.Split(strings.TrimPrefix(text, "RRULE:"), ";") {
		key, value, _ := strings.Cut(part, "=")
		switch strings.ToUpper(key) {
		case "FREQ":
			rule.Frequency = rule.frequencyOf(value)
		case "INTERVAL":
			rule.Interval = rule.numbers(value)[0]
		case "COUNT":
			rule.Count = rule.numbers(value)[0]
		case "UNTIL":
			rule.Until, rule.HasUntil = rule.until(value)
		case "WKST":
			rule.WeekStart = rule.weekdayOf(value)
		case "BYSETPOS":
			rule.BySetPos = rule.numbers(value)
		case "BYMONTH":
			rule.ByMonth = rule.numbers(value)
		case "BYMONTHDAY":
			for _, day := range rule.numbers(value) {
				if day < 0 {
					rule.ByNegative = append(rule.ByNegative, day)
				} else {
					rule.ByMonthDay = append(rule.ByMonthDay, day)
				}
			}
		case "BYYEARDAY":
			rule.ByYearDay = rule.numbers(value)
		case "BYWEEKNO":
			rule.ByWeekNo = rule.numbers(value)
		case "BYDAY", "BYWEEKDAY":
			hasWeekdays = true
			for _, day := range strings.Split(value, ",") {
				weekdays = append(weekdays, day)
			}
		}
	}
	if !(len(rule.ByWeekNo) > 0 || len(rule.ByYearDay) > 0 || len(rule.ByMonthDay) > 0 || len(rule.ByNegative) > 0 || hasWeekdays) {
		first := time.UnixMilli(start).UTC()
		switch rule.Frequency {
		case rule.yearly():
			if len(rule.ByMonth) == 0 {
				rule.ByMonth = []int{int(first.Month())}
			}
			rule.ByMonthDay = []int{first.Day()}
		case rule.monthly():
			rule.ByMonthDay = []int{first.Day()}
		case rule.weekly():
			weekdays = append(weekdays, (RecurrenceYear{}).weekdayOf(first))
		}
	}
	for _, entry := range weekdays {
		if weekday, isNumber := entry.(int); isNumber {
			rule.ByWeekday = append(rule.ByWeekday, weekday)
			continue
		}
		weekday, nth := rule.nthWeekday(entry.(string))
		if nth == 0 || rule.Frequency > rule.monthly() {
			rule.ByWeekday = append(rule.ByWeekday, weekday)
		} else {
			rule.ByNthWeekday = append(rule.ByNthWeekday, [2]int{weekday, nth})
		}
	}
	return rule
}

// Between answers every occurrence from after through before (inclusive),
// in order.
func (rule RecurrenceRule) Between(after int64, before int64) []int64 {
	results := []int64{}
	first := time.UnixMilli(rule.Start).UTC()
	year, month, day := first.Year(), int(first.Month()), first.Day()
	clock := int64(first.Hour())*3_600_000 + int64(first.Minute())*60_000 + int64(first.Second())*1000 + rule.Start%1000
	count := rule.Count
	years := map[[2]int]RecurrenceYear{}
	for {
		key := [2]int{year, month}
		info, known := years[key]
		if !known {
			info = RecurrenceYear{}.Built(rule, year, month)
			years[key] = info
		}
		dayset, start, end := rule.dayset(info, year, month, day)
		for index := start; index < end; index++ {
			if dayset[index] >= 0 && rule.filtered(info, dayset[index]) {
				dayset[dayset[index]] = -1
			}
		}
		candidates := []int64{}
		if len(rule.BySetPos) > 0 {
			candidates = rule.positions(info, dayset, start, end, clock)
		} else {
			for index := start; index < end; index++ {
				if dayset[index] >= 0 {
					candidates = append(candidates, (info.Ordinal+int64(dayset[index]))*86_400_000+clock)
				}
			}
		}
		for _, candidate := range candidates {
			if rule.HasUntil && candidate > rule.Until {
				return results
			}
			if candidate < rule.Start {
				continue
			}
			if candidate > before {
				return results
			}
			if candidate >= after {
				results = append(results, candidate)
			}
			if count > 0 {
				count--
				if count == 0 {
					return results
				}
			}
		}
		year, month, day = rule.advance(year, month, day)
		if year > rule.lastYear() {
			return results
		}
	}
}

// dayset answers the candidate day indexes of one period (-1 for none) and
// the range of the period within them.
func (rule RecurrenceRule) dayset(info RecurrenceYear, year int, month int, day int) ([]int, int, int) {
	set := make([]int, info.Length+7)
	for index := range set {
		set[index] = -1
	}
	switch rule.Frequency {
	case rule.yearly():
		for index := 0; index < info.Length; index++ {
			set[index] = index
		}
		return set, 0, info.Length
	case rule.monthly():
		start, end := info.MonthStarts[month-1], info.MonthStarts[month]
		for index := start; index < end; index++ {
			set[index] = index
		}
		return set, start, end
	case rule.weekly():
		index := rule.dayIndex(info, year, month, day)
		start := index
		for step := 0; step < 7; step++ {
			set[index] = index
			index++
			if info.weekdayAt(index) == rule.WeekStart {
				break
			}
		}
		return set, start, index
	}
	index := rule.dayIndex(info, year, month, day)
	set[index] = index
	return set, index, index + 1
}

// filtered reports whether the BY rules drop one candidate day.
func (rule RecurrenceRule) filtered(info RecurrenceYear, day int) bool {
	at := func(values []int, index int) int {
		if index < 0 || index >= len(values) {
			return 0
		}
		return values[index]
	}
	if len(rule.ByMonth) > 0 && !info.includes(rule.ByMonth, at(info.Months, day)) {
		return true
	}
	if len(rule.ByWeekNo) > 0 && !(day < len(info.WeekNumbers) && info.WeekNumbers[day]) {
		return true
	}
	if len(rule.ByWeekday) > 0 && !info.includes(rule.ByWeekday, info.weekdayAt(day)) {
		return true
	}
	if len(info.NthWeekdays) > 0 && !(day < len(info.NthWeekdays) && info.NthWeekdays[day]) {
		return true
	}
	if (len(rule.ByMonthDay) > 0 || len(rule.ByNegative) > 0) && !info.includes(rule.ByMonthDay, at(info.MonthDays, day)) && !info.includes(rule.ByNegative, at(info.NegativeDay, day)) {
		return true
	}
	if len(rule.ByYearDay) > 0 {
		if day < info.Length {
			return !info.includes(rule.ByYearDay, day+1) && !info.includes(rule.ByYearDay, -info.Length+day)
		}
		return !info.includes(rule.ByYearDay, day+1-info.Length) && !info.includes(rule.ByYearDay, -info.NextLength+day-info.Length)
	}
	return false
}

// positions answers the BYSETPOS picks of one period, in order.
func (rule RecurrenceRule) positions(info RecurrenceYear, dayset []int, start int, end int, clock int64) []int64 {
	days := []int{}
	for index := start; index < end; index++ {
		if dayset[index] >= 0 {
			days = append(days, dayset[index])
		}
	}
	picked := []int64{}
	for _, position := range rule.BySetPos {
		index := position - 1
		if position < 0 {
			index = len(days) + position
		}
		if index < 0 || index >= len(days) {
			continue
		}
		picked = append(picked, (info.Ordinal+int64(days[index]))*86_400_000+clock)
	}
	sort.SliceStable(picked, func(left, right int) bool { return picked[left] < picked[right] })
	return picked
}

// advance answers the year, month and day the next period starts on.
func (rule RecurrenceRule) advance(year int, month int, day int) (int, int, int) {
	switch rule.Frequency {
	case rule.yearly():
		return year + rule.Interval, month, day
	case rule.monthly():
		month += rule.Interval
		if month > 12 {
			year += month / 12
			month = month % 12
			if month == 0 {
				month = 12
				year--
			}
		}
		return year, month, day
	case rule.weekly():
		weekday := (RecurrenceYear{}).weekdayOf(time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC))
		if rule.WeekStart > weekday {
			day += -(weekday + 1 + (6 - rule.WeekStart)) + rule.Interval*7
		} else {
			day += -(weekday - rule.WeekStart) + rule.Interval*7
		}
	default:
		day += rule.Interval
	}
	return rule.fixed(year, month, day)
}

// fixed carries days past a month end into the following months.
func (rule RecurrenceRule) fixed(year int, month int, day int) (int, int, int) {
	length := func() int { return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day() }
	for day > length() {
		day -= length()
		month++
		if month == 13 {
			month = 1
			year++
			if year > rule.lastYear() {
				return year, month, day
			}
		}
	}
	return year, month, day
}

func (RecurrenceRule) dayIndex(info RecurrenceYear, year int, month int, day int) int {
	return int(time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix()/86_400 - info.Ordinal)
}

func (rule RecurrenceRule) until(value string) (int64, bool) {
	parts := regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})(T(\d{2})(\d{2})(\d{2})Z?)?$`).FindStringSubmatch(value)
	if parts == nil {
		return 0, false
	}
	number := func(text string) int {
		parsed, _ := strconv.Atoi(text)
		return parsed
	}
	return time.Date(number(parts[1]), time.Month(number(parts[2])), number(parts[3]), number(parts[5]), number(parts[6]), number(parts[7]), 0, time.UTC).UnixMilli(), true
}

func (RecurrenceRule) numbers(value string) []int {
	numbers := []int{}
	for _, part := range strings.Split(value, ",") {
		number, _ := strconv.Atoi(strings.TrimPrefix(part, "+"))
		numbers = append(numbers, number)
	}
	return numbers
}

func (rule RecurrenceRule) nthWeekday(text string) (int, int) {
	if len(text) == 2 {
		return rule.weekdayOf(text), 0
	}
	nth, _ := strconv.Atoi(strings.TrimPrefix(text[:len(text)-2], "+"))
	return rule.weekdayOf(text[len(text)-2:]), nth
}

func (RecurrenceRule) weekdayOf(text string) int {
	return strings.Index("MOTUWETHFRSASU", strings.ToUpper(text)) / 2
}

func (rule RecurrenceRule) frequencyOf(text string) int {
	return map[string]int{"YEARLY": rule.yearly(), "MONTHLY": rule.monthly(), "WEEKLY": rule.weekly(), "DAILY": rule.daily()}[strings.ToUpper(text)]
}

func (RecurrenceRule) yearly() int { return 0 }

func (RecurrenceRule) monthly() int { return 1 }

func (RecurrenceRule) weekly() int { return 2 }

func (RecurrenceRule) daily() int { return 3 }

func (RecurrenceRule) lastYear() int { return 9999 }
