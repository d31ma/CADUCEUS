package blobs

import "regexp"

type KeyRules struct{}

func (rules KeyRules) Check(key Key) error {
	err := rules.checkDomain(key.DomainId)
	if err != nil {
		return err
	}
	return rules.checkName(key.Name)
}

func (rules KeyRules) checkDomain(domainId string) error {
	if len(domainId) > rules.longestDomain() || !regexp.MustCompile(rules.domainPattern()).MatchString(domainId) {
		return BadKey{Part: "domain", Value: domainId}
	}
	return nil
}

func (rules KeyRules) checkName(name string) error {
	if !regexp.MustCompile(rules.namePattern()).MatchString(name) {
		return BadKey{Part: "name", Value: name}
	}
	return nil
}

func (KeyRules) longestDomain() int {
	return 253
}

func (KeyRules) domainPattern() string {
	return `^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`
}

func (KeyRules) namePattern() string {
	return `^[A-Za-z0-9_-]{1,128}$`
}
