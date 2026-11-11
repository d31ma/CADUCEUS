package dnscheck

import "slices"

type dkimCheck struct {
	choice stateChoice
	tags   tagList
}

func (check dkimCheck) state(records []string) string {
	valid := slices.ContainsFunc(records, check.valid)
	return check.choice.missingUnless(check.choice.found(records), check.choice.okWhen(valid))
}

func (check dkimCheck) valid(record string) bool {
	values := check.tags.values(record)
	hasVersion := values[check.versionTag()] == check.version()
	hasKey := values[check.keyTag()] != ""
	return check.choice.both(hasVersion, hasKey)
}

func (dkimCheck) versionTag() string {
	return "v"
}

func (dkimCheck) version() string {
	return "DKIM1"
}

func (dkimCheck) keyTag() string {
	return "p"
}
