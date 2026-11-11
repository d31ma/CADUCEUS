import (
	"slices"
	"strings"
)

// @Service
type UsernameRuleService struct{}

func (rules *UsernameRuleService) Cleaned(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func (rules *UsernameRuleService) Problem(username string) string {
	cleaned := rules.Cleaned(username)
	problem := rules.roleProblem(cleaned)
	problem = rules.shapeProblem(rules.wellShaped(cleaned), problem)
	return rules.emptyProblem(cleaned, problem)
}

func (rules *UsernameRuleService) PasswordProblem(password string) string {
	return rules.shortProblem(len([]rune(password)) < rules.passwordLength())
}

func (rules *UsernameRuleService) wellShaped(cleaned string) bool {
	return len(cleaned) <= rules.maximumLength() &&
		!strings.ContainsFunc(cleaned, rules.notAllowed) &&
		strings.Trim(cleaned, rules.punctuation()) == cleaned &&
		!strings.Contains(cleaned, "..")
}

func (rules *UsernameRuleService) notAllowed(character rune) bool {
	return !strings.ContainsRune(rules.allowedCharacters(), character)
}

func (rules *UsernameRuleService) roleProblem(cleaned string) string {
	if !slices.Contains(rules.roleNames(), cleaned) {
		return ""
	}
	return cleaned + " is kept for system mail. Choose another username, or add " + cleaned + " as an alias."
}

func (rules *UsernameRuleService) shapeProblem(wellShaped bool, problem string) string {
	if wellShaped {
		return problem
	}
	return "Use 1 to 64 letters, digits, dots, dashes or underscores. Start and end with a letter or digit."
}

func (rules *UsernameRuleService) emptyProblem(cleaned string, problem string) string {
	if cleaned != "" {
		return problem
	}
	return "Type a username, like dan."
}

func (rules *UsernameRuleService) shortProblem(short bool) string {
	if !short {
		return ""
	}
	return "Use at least 12 characters."
}

func (rules *UsernameRuleService) roleNames() []string {
	return []string{"postmaster", "abuse", "hostmaster", "webmaster", "mailer-daemon", "noreply", "no-reply", "security"}
}

func (rules *UsernameRuleService) allowedCharacters() string {
	return "abcdefghijklmnopqrstuvwxyz0123456789._-"
}

func (rules *UsernameRuleService) punctuation() string {
	return "._-"
}

func (rules *UsernameRuleService) maximumLength() int {
	return 64
}

func (rules *UsernameRuleService) passwordLength() int {
	return 12
}
