package login

import (
	"cmp"
	"slices"

	"mesail.dev/packages/addresses"
	"mesail.dev/packages/oidc"
)

type DomainRules struct{}

func (rules DomainRules) ForSignIn(person Person, operator bool, wanted string) (string, error) {
	err := rules.signInAllowed(person)
	if err != nil {
		return "", err
	}
	return rules.signInDomain(person, operator, wanted)
}

func (rules DomainRules) Kept(person Person, domainId string) error {
	err := rules.active(person)
	if err != nil {
		return err
	}
	return rules.stillMember(person, domainId)
}

func (rules DomainRules) ForSwitch(person Person, wanted string) (string, error) {
	err := rules.active(person)
	if err != nil {
		return "", err
	}
	return rules.switchDomain(person, wanted)
}

func (rules DomainRules) InstallWideFor(operator bool, domainId string) error {
	return rules.installWideAllowed(operator, domainId)
}

func (DomainRules) installWideAllowed(operator bool, domainId string) error {
	if (oidc.InstallWide{}).Matches(domainId) && !operator {
		return NoDomain{}
	}
	return nil
}

func (rules DomainRules) Roles(person Person, operator bool, domainId string) []string {
	return slices.Concat(rules.memberRole(domainId), rules.adminRole(person, domainId), rules.operatorRole(operator))
}

func (rules DomainRules) Address(username string, domainId string) string {
	return rules.address(username, domainId)
}

func (rules DomainRules) Choices(person Person) []DomainChoice {
	return rules.choices(person)
}

func (DomainRules) choices(person Person) []DomainChoice {
	opened := DomainRules{}.firstChoice(person)
	choices := []DomainChoice{}
	for _, domainId := range person.DomainIds {
		choices = append(choices, DomainChoice{DomainId: domainId, Admin: slices.Contains(person.AdminDomainIds, domainId), Default: domainId == opened})
	}
	return choices
}

func (rules DomainRules) signInAllowed(person Person) error {
	if rules.inactive(person) {
		return oidc.SignInRefused{}
	}
	return nil
}

func (rules DomainRules) active(person Person) error {
	if rules.inactive(person) {
		return AccessRevoked{}
	}
	return nil
}

func (DomainRules) inactive(person Person) bool {
	return person.State == PersonStates{}.Disabled() || person.State == PersonStates{}.Deleted()
}

func (rules DomainRules) signInDomain(person Person, operator bool, wanted string) (string, error) {
	if wanted == "" {
		return rules.opening(person, operator)
	}
	return rules.member(person, wanted, NotAMember{})
}

func (rules DomainRules) switchDomain(person Person, wanted string) (string, error) {
	if wanted == "" {
		return cmp.Or(rules.firstChoice(person), oidc.InstallWide{}.DomainId()), nil
	}
	return rules.member(person, wanted, NotFound{})
}

func (rules DomainRules) opening(person Person, operator bool) (string, error) {
	domainId := rules.firstChoice(person)
	if domainId != "" {
		return domainId, nil
	}
	return rules.installWide(operator)
}

func (DomainRules) installWide(operator bool) (string, error) {
	if !operator {
		return "", NoDomain{}
	}
	return oidc.InstallWide{}.DomainId(), nil
}

func (rules DomainRules) firstChoice(person Person) string {
	if slices.Contains(person.DomainIds, person.DefaultDomainId) {
		return person.DefaultDomainId
	}
	return rules.first(person.DomainIds)
}

func (DomainRules) first(domainIds []string) string {
	if len(domainIds) == 0 {
		return ""
	}
	return domainIds[0]
}

func (DomainRules) member(person Person, domainId string, missing error) (string, error) {
	if !slices.Contains(person.DomainIds, domainId) {
		return "", missing
	}
	return domainId, nil
}

func (DomainRules) stillMember(person Person, domainId string) error {
	if !(oidc.InstallWide{}).Matches(domainId) && !slices.Contains(person.DomainIds, domainId) {
		return NotAMember{}
	}
	return nil
}

func (DomainRules) memberRole(domainId string) []string {
	if (oidc.InstallWide{}).Matches(domainId) {
		return nil
	}
	return []string{oidc.RealmRoles{}.Member()}
}

func (DomainRules) adminRole(person Person, domainId string) []string {
	if !slices.Contains(person.AdminDomainIds, domainId) {
		return nil
	}
	return []string{oidc.RealmRoles{}.Admin()}
}

func (DomainRules) operatorRole(operator bool) []string {
	if !operator {
		return nil
	}
	return []string{oidc.RealmRoles{}.Operator()}
}

func (DomainRules) address(username string, domainId string) string {
	if (oidc.InstallWide{}).Matches(domainId) {
		return ""
	}
	return addresses.Address{}.Primary(username, domainId)
}
