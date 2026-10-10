import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf16"
)

// GrammarUtils keeps only well-formed grammar matches that fit the checked
// text, like normalizeGrammarMatches in client/shared/scripts/grammar-check.js.
// Offsets are UTF-16 code units, as the server and the browser count them.
type GrammarUtils struct{}

// Issues turns the server matches into the issues the panel lists.
func (GrammarUtils) Issues(matches []GrammarMatch, text string) []GrammarIssue {
	units := utf16.Encode([]rune(text))
	issues := GrammarUtils{}.fitting(matches, units)
	return issues[:min(len(issues), GrammarUtils{}.mostIssues())]
}

// ValidRule says whether a rule identifier may be stored or sent.
func (GrammarUtils) ValidRule(rule string) bool {
	return GrammarUtils{}.rulePattern().MatchString(rule)
}

// ValidRules keeps at most 100 valid rule identifiers.
func (GrammarUtils) ValidRules(rules []string) []string {
	kept := GrammarUtils{}.validOnly(rules)
	return kept[:min(len(kept), GrammarUtils{}.mostRules())]
}

// Status describes the result of one check.
func (GrammarUtils) Status(count int, detectedLanguage string) string {
	return GrammarUtils{}.statusOf(count, detectedLanguage)
}

// IssueCount describes how many issues remain after a rule was ignored.
func (GrammarUtils) IssueCount(count int) string {
	return GrammarUtils{}.statusOf(count, "")
}

func (GrammarUtils) mostIssues() int { return 100 }

func (GrammarUtils) space() string { return " " }

func (GrammarUtils) mostRules() int { return 100 }

func (GrammarUtils) rulePattern() *regexp.Regexp {
	return regexp.MustCompile(`^[A-Z0-9_-]{1,64}$`)
}

func (GrammarUtils) validOnly(rules []string) []string {
	kept := []string{}
	for _, rule := range rules {
		kept = append(kept, GrammarUtils{}.whenValid(rule)...)
	}
	return kept
}

func (GrammarUtils) whenValid(rule string) []string {
	if GrammarUtils{}.rulePattern().MatchString(rule) {
		return []string{rule}
	}
	return nil
}

func (GrammarUtils) fitting(matches []GrammarMatch, units []uint16) []GrammarIssue {
	issues := []GrammarIssue{}
	for _, match := range matches {
		issues = append(issues, GrammarUtils{}.whenFits(match, units)...)
	}
	return issues
}

func (GrammarUtils) whenFits(match GrammarMatch, units []uint16) []GrammarIssue {
	offset, length := int(match.Offset), int(match.Length)
	valid := GrammarUtils{}.ValidRule(match.RuleID)
	if !valid || float64(offset) != match.Offset || float64(length) != match.Length || offset < 0 || length < 1 || offset+length > len(units) {
		return nil
	}
	return []GrammarIssue{GrammarUtils{}.issue(match, offset, length, units)}
}

func (GrammarUtils) issue(match GrammarMatch, offset int, length int, units []uint16) GrammarIssue {
	return GrammarIssue{
		Offset:       offset,
		Length:       length,
		Excerpt:      string(utf16.Decode(units[offset : offset+length])),
		Message:      GrammarUtils{}.orDefault(GrammarUtils{}.safe(match.Message, 300), "Grammar issue"),
		ShortMessage: GrammarUtils{}.orDefault(GrammarUtils{}.safe(match.ShortMessage, 80), "Grammar"),
		Category:     GrammarUtils{}.safe(match.Category, 40),
		RuleID:       match.RuleID,
		Replacements: append([]string{}, match.Replacements[:min(len(match.Replacements), 5)]...),
	}
}

func (GrammarUtils) safe(value string, most int) string {
	space := GrammarUtils{}.space()
	spaced := regexp.MustCompile(`[\x00-\x1f\x7f]`).ReplaceAllString(value, space)
	units := utf16.Encode([]rune(strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(spaced, space))))
	return string(utf16.Decode(units[:min(len(units), most)]))
}

func (GrammarUtils) orDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (GrammarUtils) statusOf(count int, detectedLanguage string) string {
	if count == 1 {
		return "1 grammar issue."
	} else if count > 1 {
		return fmt.Sprintf("%d grammar issues.", count)
	} else if detectedLanguage == "und" {
		return "This language is not supported by the local checker."
	}
	return "No grammar issues."
}
