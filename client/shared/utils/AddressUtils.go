import (
	"regexp"
	"slices"
	"strings"
)

// AddressUtils reads mailbox addresses the way the reader shows them.
type AddressUtils struct{}

func (AddressUtils) Initials(address string) string {
	local, _, _ := strings.Cut(address, "@")
	return strings.ToUpper(AddressUtils{}.firstTwo(TextUtils{}.FirstOf(local, "?")))
}

func (AddressUtils) Normalized(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (AddressUtils) NormalizedAll(values []string) []string {
	return AddressUtils{}.normalizedAll(values)
}

func (AddressUtils) Valid(address string) string {
	return AddressUtils{}.valid(address)
}

func (AddressUtils) Trusted(sender string, trusted []string) bool {
	return AddressUtils{}.trusted(sender, trusted)
}

func (AddressUtils) Participant(sender string, recipients []string, own string) string {
	others := AddressUtils{}.others(AddressUtils{}.normalizedAll(recipients), own)
	return AddressUtils{}.valid(AddressUtils{}.participant(AddressUtils{}.Normalized(sender), TextUtils{}.First(others), own))
}

func (AddressUtils) trusted(sender string, trusted []string) bool {
	return sender != "" && slices.Contains(trusted, sender)
}

func (AddressUtils) normalizedAll(values []string) []string {
	normalized := []string{}
	for _, value := range values {
		normalized = append(normalized, AddressUtils{}.Normalized(value))
	}
	return normalized
}

func (AddressUtils) others(addresses []string, own string) []string {
	return slices.DeleteFunc(addresses, func(address string) bool { return AddressUtils{}.excluded(address, own) })
}

func (AddressUtils) excluded(address string, own string) bool {
	return address == "" || address == own
}

func (AddressUtils) isOther(sender string, own string) bool {
	return sender != "" && sender != own
}

func (AddressUtils) firstTwo(text string) string {
	runes := []rune(text)
	return string(runes[:min(2, len(runes))])
}

func (AddressUtils) valid(address string) string {
	if regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`).MatchString(address) {
		return address
	}
	return ""
}

func (AddressUtils) participant(sender string, recipient string, own string) string {
	switch {
	case AddressUtils{}.isOther(sender, own):
		return sender
	case recipient != "":
		return recipient
	}
	return sender
}
