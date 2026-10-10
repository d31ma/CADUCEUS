import (
	"regexp"
	"strings"
)

// MailAddresses finds mailbox addresses in free text with the address
// pattern the JavaScript records shared, ignoring display names and
// brackets.
type MailAddresses struct{}

func (MailAddresses) pattern() *regexp.Regexp {
	return regexp.MustCompile("(?i)[A-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?(?:\\.[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?)+")
}

// Found answers every address in String(value), lower-cased, in order.
func (addresses MailAddresses) Found(value any) []string {
	found := addresses.pattern().FindAllString(TextValue{}.Of(value), -1)
	for index, address := range found {
		found[index] = strings.ToLower(address)
	}
	return found
}

// Unique answers values without repeats, keeping their first order.
func (MailAddresses) Unique(values []string) []string {
	seen := map[string]bool{}
	unique := []string{}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			unique = append(unique, value)
		}
	}
	return unique
}
