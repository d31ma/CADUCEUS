package dnscheck

import "slices"

type mxCheck struct {
	choice stateChoice
	hosts  hostName
}

func (check mxCheck) state(hosts []string, mailHost string) string {
	matched := slices.Contains(check.normalizeAll(hosts), check.hosts.normalize(mailHost))
	return check.choice.missingUnless(check.choice.found(hosts), check.choice.okWhen(matched))
}

func (check mxCheck) normalizeAll(hosts []string) []string {
	normalized := []string{}
	for _, host := range hosts {
		normalized = append(normalized, check.hosts.normalize(host))
	}
	return normalized
}
