package recipients

import "strings"

type DomainNames struct{}

func (names DomainNames) Normalize(raw string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(raw))
	return name, names.refusal(names.valid(name), raw)
}

func (names DomainNames) valid(name string) bool {
	labels := strings.Split(name, names.dot())
	return len(name) <= names.maxLength() && len(labels) > 1 && names.allLabelsValid(labels)
}

func (names DomainNames) allLabelsValid(labels []string) bool {
	for _, label := range labels {
		if !names.validLabel(label) {
			return false
		}
	}
	return true
}

func (names DomainNames) validLabel(label string) bool {
	return len(label) >= 1 && len(label) <= names.maxLabelLength() && strings.Trim(label, names.labelCharacters()) == "" && names.hyphensInside(label)
}

func (names DomainNames) hyphensInside(label string) bool {
	return !strings.HasPrefix(label, names.hyphen()) && !strings.HasSuffix(label, names.hyphen())
}

func (names DomainNames) refusal(valid bool, raw string) error {
	if valid {
		return nil
	}
	return InvalidDomainName{Name: raw}
}

func (DomainNames) dot() string {
	return "."
}

func (DomainNames) hyphen() string {
	return "-"
}

func (DomainNames) labelCharacters() string {
	return "abcdefghijklmnopqrstuvwxyz0123456789-"
}

func (DomainNames) maxLength() int {
	return 253
}

func (DomainNames) maxLabelLength() int {
	return 63
}
