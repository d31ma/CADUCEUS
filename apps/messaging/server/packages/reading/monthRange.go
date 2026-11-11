package reading

import (
	"time"

	"mesail.dev/packages/core"
)

type monthRange struct{}

func (months monthRange) back(now time.Time, createdMonth string) []string {
	return months.listed(now.UTC(), months.count(now.UTC(), createdMonth))
}

func (months monthRange) count(now time.Time, createdMonth string) int {
	created, err := time.Parse(months.layout(), createdMonth)
	if err != nil {
		return months.most()
	}
	return min(max(months.between(created, now)+1, 1), months.most())
}

func (months monthRange) between(from time.Time, to time.Time) int {
	return (to.Year()-from.Year())*months.perYear() + int(to.Month()) - int(from.Month())
}

func (monthRange) listed(now time.Time, count int) []string {
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	listed := []string{}
	for index := 0; index < count; index++ {
		listed = append(listed, core.MessageIds{}.Month(first.AddDate(0, -index, 0)))
	}
	return listed
}

func (monthRange) layout() string {
	return "200601"
}

func (monthRange) most() int {
	return 24
}

func (monthRange) perYear() int {
	return 12
}
