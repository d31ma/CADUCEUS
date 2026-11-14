import (
	"cmp"
	"slices"
)

// @Service
type MembershipService struct {
	// @Needs
	settings *MemberSettingsStoreService
}

func (service *MembershipService) Split(domainId string, personIds []string) (Membership, error) {
	people := service.distinct(personIds)
	active, failure := service.states(domainId, people)
	return Membership{Active: service.withState(active, people, true), Outsiders: service.withState(active, people, false)}, failure
}

func (service *MembershipService) distinct(personIds []string) []string {
	people := slices.Clone(personIds)
	slices.Sort(people)
	return slices.Compact(people)
}

func (service *MembershipService) states(domainId string, personIds []string) (map[string]bool, error) {
	active := map[string]bool{}
	failures := []error{}
	for _, personId := range personIds {
		record, failure := service.settings.Get(domainId, personId)
		active[personId], failures = record.State == service.activeState(), append(failures, failure)
	}
	return active, cmp.Or(failures...)
}

func (service *MembershipService) withState(active map[string]bool, personIds []string, wanted bool) []string {
	return slices.DeleteFunc(slices.Clone(personIds), func(personId string) bool { return active[personId] != wanted })
}

func (service *MembershipService) activeState() string {
	return "active"
}
