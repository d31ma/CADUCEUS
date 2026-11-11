package dnscheck

import (
	"slices"
	"strings"
)

type dmarcCheck struct {
	choice stateChoice
	tags   tagList
}

func (check dmarcCheck) state(records []string) string {
	valid := slices.ContainsFunc(records, check.valid)
	return check.choice.missingUnless(check.choice.found(records), check.choice.okWhen(valid))
}

func (check dmarcCheck) valid(record string) bool {
	startsWithVersion := check.tags.first(record) == check.version()
	policy := strings.ToLower(check.tags.values(record)[check.policyTag()])
	return check.choice.both(startsWithVersion, slices.Contains(check.policies(), policy))
}

func (dmarcCheck) version() string {
	return "v=DMARC1"
}

func (dmarcCheck) policyTag() string {
	return "p"
}

func (dmarcCheck) policies() []string {
	return []string{"none", "quarantine", "reject"}
}
