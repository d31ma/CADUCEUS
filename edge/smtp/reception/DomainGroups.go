package reception

import (
	"maps"
	"slices"
)

type DomainGroups struct{}

func (groups DomainGroups) ByDomain(recipients []Recipient) []DomainGroup {
	collected := groups.collect(recipients)
	return groups.ordered(collected, slices.Sorted(maps.Keys(collected)))
}

func (DomainGroups) collect(recipients []Recipient) map[string][]string {
	collected := map[string][]string{}
	for _, recipient := range recipients {
		collected[recipient.DomainId] = append(collected[recipient.DomainId], recipient.Address)
	}
	return collected
}

func (DomainGroups) ordered(collected map[string][]string, domains []string) []DomainGroup {
	groups := make([]DomainGroup, 0, len(domains))
	for _, domain := range domains {
		groups = append(groups, DomainGroup{DomainId: domain, Recipients: slices.Sorted(slices.Values(collected[domain]))})
	}
	return groups
}
