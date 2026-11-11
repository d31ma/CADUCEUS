package addresses

import "regexp"

type DomainName struct{}

func (name DomainName) Normalized(input string) (string, error) {
	value, err := nameText{}.lowered(NameKinds{}.Domain(), input)
	if err != nil {
		return "", err
	}
	err = name.check(value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (name DomainName) check(value string) error {
	err := name.checkLength(value)
	if err != nil {
		return err
	}
	return name.checkLabels(value)
}

func (DomainName) maximumLength() int {
	return 253
}

func (DomainName) labels() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)
}

func (name DomainName) checkLength(value string) error {
	if len(value) > name.maximumLength() {
		return InvalidName{Kind: NameKinds{}.Domain(), Reason: "must be at most 253 characters"}
	}
	return nil
}

func (name DomainName) checkLabels(value string) error {
	if !name.labels().MatchString(value) {
		return InvalidName{Kind: NameKinds{}.Domain(), Reason: "must be labels of 1 to 63 characters from a-z, 0-9 and - joined by single dots, with no - at the start or end of a label"}
	}
	return nil
}
