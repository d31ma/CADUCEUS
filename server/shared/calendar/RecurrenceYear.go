import "time"

// RecurrenceYear holds the per-day masks of one year that recurrence
// expansion filters candidate days with, as the rrule library (a port of
// python-dateutil) built them. Day indexes count from January 1; masks run
// seven days past the year end so a week can spill into the next year.
type RecurrenceYear struct {
	Length      int
	NextLength  int
	Ordinal     int64
	Weekday     int
	Months      []int
	MonthDays   []int
	NegativeDay []int
	Weekdays    []int
	MonthStarts []int
	WeekNumbers []bool
	NthWeekdays []bool
}

// Built answers the masks of a year (and, for nth-weekday rules, of the
// month or months the rule ranges over).
func (RecurrenceYear) Built(rule RecurrenceRule, year int, month int) RecurrenceYear {
	info := RecurrenceYear{}
	first := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	info.Length = info.lengthOf(year)
	info.NextLength = info.lengthOf(year + 1)
	info.Ordinal = first.Unix() / (24 * 60 * 60)
	info.Weekday = info.weekdayOf(first)
	info.Months, info.MonthDays, info.NegativeDay, info.MonthStarts = info.monthMasks(info.Length)
	info.Weekdays = []int{}
	for index := 0; index < info.Length+14; index++ {
		info.Weekdays = append(info.Weekdays, (info.Weekday+index)%7)
	}
	if len(rule.ByWeekNo) > 0 {
		info.WeekNumbers = info.weekNumbers(rule, year)
	}
	if len(rule.ByNthWeekday) > 0 {
		info.NthWeekdays = info.nthWeekdays(rule, month)
	}
	return info
}

func (info RecurrenceYear) weekNumbers(rule RecurrenceRule, year int) []bool {
	mask := make([]bool, info.Length+21)
	mark := func(index int) {
		if index >= 0 && index < len(mask) {
			mask[index] = true
		}
	}
	firstWeekStart := info.modulo(7-info.Weekday+rule.WeekStart, 7)
	weekOneStart := firstWeekStart
	weekYearLength := 0
	if weekOneStart >= 4 {
		weekOneStart = 0
		weekYearLength = info.Length + info.modulo(info.Weekday-rule.WeekStart, 7)
	} else {
		weekYearLength = info.Length - weekOneStart
	}
	weeks := weekYearLength/7 + info.modulo(weekYearLength, 7)/4
	for _, number := range rule.ByWeekNo {
		if number < 0 {
			number += weeks + 1
		}
		if !(number > 0 && number <= weeks) {
			continue
		}
		index := weekOneStart
		if number > 1 {
			index = weekOneStart + (number-1)*7
			if weekOneStart != firstWeekStart {
				index -= 7 - firstWeekStart
			}
		}
		for step := 0; step < 7; step++ {
			mark(index)
			index++
			if info.weekdayAt(index) == rule.WeekStart {
				break
			}
		}
	}
	if info.includes(rule.ByWeekNo, 1) {
		index := weekOneStart + weeks*7
		if weekOneStart != firstWeekStart {
			index -= 7 - firstWeekStart
		}
		if index < info.Length {
			for step := 0; step < 7; step++ {
				mark(index)
				index++
				if info.weekdayAt(index) == rule.WeekStart {
					break
				}
			}
		}
	}
	if weekOneStart != 0 {
		lastWeeks := -1
		if !info.includes(rule.ByWeekNo, -1) {
			lastWeekday := info.weekdayOf(time.Date(year-1, time.January, 1, 0, 0, 0, 0, time.UTC))
			lastWeekOneStart := info.modulo(7-lastWeekday+rule.WeekStart, 7)
			lastLength := info.lengthOf(year - 1)
			weekStart := 0
			if lastWeekOneStart >= 4 {
				weekStart = lastLength + info.modulo(lastWeekday-rule.WeekStart, 7)
			} else {
				weekStart = info.Length - weekOneStart
			}
			lastWeeks = 52 + info.modulo(weekStart, 7)/4
		}
		if info.includes(rule.ByWeekNo, lastWeeks) {
			for index := 0; index < weekOneStart; index++ {
				mark(index)
			}
		}
	}
	return mask
}

func (info RecurrenceYear) nthWeekdays(rule RecurrenceRule, month int) []bool {
	ranges := [][2]int{}
	if rule.Frequency == rule.yearly() {
		if len(rule.ByMonth) > 0 {
			for _, chosen := range rule.ByMonth {
				ranges = append(ranges, [2]int{info.MonthStarts[chosen-1], info.MonthStarts[chosen]})
			}
		} else {
			ranges = append(ranges, [2]int{0, info.Length})
		}
	} else if rule.Frequency == rule.monthly() {
		ranges = append(ranges, [2]int{info.MonthStarts[month-1], info.MonthStarts[month]})
	}
	if len(ranges) == 0 {
		return nil
	}
	mask := make([]bool, info.Length)
	for _, bounds := range ranges {
		first, last := bounds[0], bounds[1]-1
		for _, nth := range rule.ByNthWeekday {
			weekday, number := nth[0], nth[1]
			index := 0
			if number < 0 {
				index = last + (number+1)*7
				index -= info.modulo(info.weekdayAt(index)-weekday, 7)
			} else {
				index = first + (number-1)*7
				index += info.modulo(7-info.weekdayAt(index)+weekday, 7)
			}
			if first <= index && index <= last {
				mask[index] = true
			}
		}
	}
	return mask
}

// monthMasks answers, per day of the year (and seven days on), its month,
// its day of the month, its day counted back from the month end, and the
// first day index of every month.
func (RecurrenceYear) monthMasks(length int) ([]int, []int, []int, []int) {
	lengths := []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if length == 366 {
		lengths[1] = 29
	}
	months, days, negative, starts := []int{}, []int{}, []int{}, []int{0}
	for index, size := range lengths {
		for day := 1; day <= size; day++ {
			months = append(months, index+1)
			days = append(days, day)
			negative = append(negative, day-size-1)
		}
		starts = append(starts, starts[len(starts)-1]+size)
	}
	for day := 1; day <= 7; day++ {
		months = append(months, 1)
		days = append(days, day)
		negative = append(negative, day-32)
	}
	return months, days, negative, starts
}

func (info RecurrenceYear) weekdayAt(index int) int {
	if index < 0 || index >= len(info.Weekdays) {
		return -1
	}
	return info.Weekdays[index]
}

func (RecurrenceYear) weekdayOf(day time.Time) int {
	return (int(day.Weekday()) + 6) % 7
}

func (RecurrenceYear) lengthOf(year int) int {
	if (year%4 == 0 && year%100 != 0) || year%400 == 0 {
		return 366
	}
	return 365
}

func (RecurrenceYear) modulo(value int, divisor int) int {
	return ((value % divisor) + divisor) % divisor
}

func (RecurrenceYear) includes(values []int, value int) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
