import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf16"
)

// PreferenceChecks checks the display-only labels and lists of a caller's
// preference update. They are rendering preferences, never authorization
// rules.
type PreferenceChecks struct{}

// MailboxNames answers the trimmed display names keyed by normalized domain,
// or why they are refused. A blank name restores the domain label.
func (checks PreferenceChecks) MailboxNames(value any, assignedDomains []string) (map[string]any, *PreferenceProblem) {
	input, isObject := RecordValues{}.Object(value)
	if !isObject {
		return nil, &PreferenceProblem{Message: "mailboxNames must be an object"}
	}
	allowed := PreferenceChoices{}.Domains(assignedDomains)
	seen := map[string]bool{}
	names := map[string]any{}
	for _, rawDomain := range (PreferenceChoices{}).Keys(input) {
		domain := Security{}.NormalizeDomain(rawDomain)
		if domain == "" || !(PreferenceChoices{}).Includes(allowed, domain) {
			return nil, &PreferenceProblem{Message: "Mailbox domain is not assigned to this user", Forbidden: true}
		}
		rawName, isText := input[rawDomain].(string)
		if !isText {
			return nil, &PreferenceProblem{Message: "Mailbox name must be text"}
		}
		name := Security{}.Trim(rawName)
		if name == "" {
			continue
		}
		if regexp.MustCompile(`[\x00\r\n]`).MatchString(name) || len(utf16.Encode([]rune(name))) > checks.maximumNameLength() {
			return nil, &PreferenceProblem{Message: fmt.Sprintf("Mailbox name must be 1 to %d printable characters", checks.maximumNameLength())}
		}
		identity := strings.ToLower(name)
		if seen[identity] {
			return nil, &PreferenceProblem{Message: "Mailbox names must be unique"}
		}
		seen[identity] = true
		names[domain] = name
	}
	return names, nil
}

func (PreferenceChecks) maximumNameLength() int { return 64 }

// TrustedImageSenders answers the distinct normalized sender addresses, or
// why the list is refused. The list is capped so one profile cannot grow
// without bound.
func (checks PreferenceChecks) TrustedImageSenders(value any) ([]string, *PreferenceProblem) {
	list, isList := RecordValues{}.List(value)
	if !isList {
		return nil, &PreferenceProblem{Message: "trustedImageSenders must be an array"}
	}
	if len(list) > checks.maximumTrustedSenders() {
		return nil, &PreferenceProblem{Message: fmt.Sprintf("trustedImageSenders may contain at most %d addresses", checks.maximumTrustedSenders())}
	}
	for _, entry := range list {
		text, isText := entry.(string)
		if !isText || (Security{}).NormalizeEmailAddress(text) == "" {
			return nil, &PreferenceProblem{Message: "Trusted image sender must be an email address"}
		}
	}
	return PreferenceChoices{}.Addresses(list), nil
}

func (PreferenceChecks) maximumTrustedSenders() int { return 500 }
