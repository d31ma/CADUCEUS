package dnscheck

import (
	"slices"
	"strings"
)

type spfCheck struct {
	choice stateChoice
}

func (check spfCheck) state(records []string, mechanism string) string {
	spf := check.spfRecords(records)
	single := len(spf) == check.allowedCount()
	matched := check.hasTerm(strings.Join(spf, check.space()), mechanism)
	return check.choice.missingUnless(check.choice.found(spf), check.choice.okWhen(check.choice.both(single, matched)))
}

func (check spfCheck) spfRecords(records []string) []string {
	spf := []string{}
	for _, record := range records {
		spf = append(spf, check.keepIfSpf(record)...)
	}
	return spf
}

func (check spfCheck) keepIfSpf(record string) []string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(record)), check.prefix()) {
		return []string{record}
	}
	return nil
}

func (check spfCheck) hasTerm(record string, mechanism string) bool {
	return slices.Contains(strings.Fields(strings.ToLower(record)), strings.ToLower(mechanism))
}

func (spfCheck) prefix() string {
	return "v=spf1"
}

func (spfCheck) allowedCount() int {
	return 1
}

func (spfCheck) space() string {
	return " "
}
