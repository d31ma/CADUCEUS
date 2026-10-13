import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// GrammarCheck is the bounded standalone grammar checker with a
// LanguageTool-compatible result shape. It leaves spelling to the browser and
// reports grammar, punctuation, casing and style issues only. Offsets and
// lengths count UTF-16 code units, as the JavaScript checker's did.
type GrammarCheck struct{}

// MaximumTextLength is the longest text a check accepts, in UTF-16 units.
func (GrammarCheck) MaximumTextLength() int { return 20_000 }

// IsLanguage reports whether value is a language the checker accepts.
func (GrammarCheck) IsLanguage(value any) bool {
	text, isText := value.(string)
	return isText && (text == "auto" || text == "en-US" || text == "en-GB")
}

// DisabledRules answers the distinct rule identifiers of value, or false when
// it is not a list of at most 100 valid identifiers.
func (check GrammarCheck) DisabledRules(value any) ([]string, bool) {
	list, isList := value.([]any)
	if !isList || len(list) > 100 {
		return nil, false
	}
	rules := []string{}
	for _, item := range list {
		rule, isText := item.(string)
		if !isText || !check.ruleId().MatchString(rule) {
			return nil, false
		}
		if !(TextValue{}).Includes(rules, rule) {
			rules = append(rules, rule)
		}
	}
	return rules, true
}

// Check answers the issues of text in the language (auto detects it).
func (check GrammarCheck) Check(text string, language string, disabledRules []string) GrammarResult {
	if !check.IsLanguage(language) {
		language = "auto"
	}
	detected := check.detectLanguage(text, language)
	disabled := map[string]bool{}
	valid := 0
	for _, rule := range disabledRules {
		if check.ruleId().MatchString(rule) && valid < 100 {
			disabled[rule] = true
			valid++
		}
	}
	if detected == "und" {
		return GrammarResult{Language: language, DetectedLanguage: detected, Matches: []GrammarMatch{}}
	}
	scan := &GrammarScan{text: text, units: check.unitOffsets(text), disabled: disabled}
	scan.length = scan.units[len(text)]
	check.scanRules(scan)
	return GrammarResult{Language: language, DetectedLanguage: detected, Matches: check.nonOverlapping(scan.matches)}
}

func (check GrammarCheck) scanRules(scan *GrammarScan) {
	space := check.space()
	check.repeatedWords(scan)
	check.lowercaseI(scan)
	check.each(scan, `[ \t]{2,}`, func(match []int, group func(int) string) {
		check.add(scan, match[0], match[1], "MULTIPLE_SPACES", "Use a single space here.", []string{" "}, "STYLE", "style")
	})
	check.each(scan, space+`+([,!?;:])`, func(match []int, group func(int) string) {
		check.add(scan, match[0], match[1], "SPACE_BEFORE_PUNCTUATION", "Remove the space before punctuation.", []string{group(1)}, "PUNCTUATION", "grammar")
	})
	check.each(scan, `([.!?])([A-Za-z])`, func(match []int, group func(int) string) {
		check.add(scan, match[0], match[1], "MISSING_SPACE_AFTER_PUNCTUATION", "Add a space after punctuation.", []string{group(1) + " " + group(2)}, "PUNCTUATION", "grammar")
	})
	check.each(scan, `(?i)\b(could|should|would|must|might) of\b`, func(match []int, group func(int) string) {
		check.add(scan, match[0], match[1], "MODAL_OF", "Use “"+group(1)+" have” instead.", []string{group(1) + " have"}, "GRAMMAR", "grammar")
	})
	check.each(scan, `(?i)\b(your) welcome\b`, func(match []int, group func(int) string) {
		check.add(scan, match[0], match[1], "YOUR_WELCOME", "Use “you’re” for “you are”.", []string{check.preserveCase(group(1), "you're") + " welcome"}, "GRAMMAR", "grammar")
	})
	check.each(scan, `(?i)\b(he|she|it|this|that) (are|were|have)\b`, func(match []int, group func(int) string) {
		verb := map[string]string{"are": "is", "were": "was", "have": "has"}[strings.ToLower(group(2))]
		check.add(scan, match[0], match[1], "SINGULAR_VERB_AGREEMENT", "Use a singular verb with this subject.", []string{group(1) + " " + verb}, "GRAMMAR", "grammar")
	})
	check.each(scan, `(?i)\b(they|we|you) (is|was|has)\b`, func(match []int, group func(int) string) {
		verb := map[string]string{"is": "are", "was": "were", "has": "have"}[strings.ToLower(group(2))]
		check.add(scan, match[0], match[1], "PLURAL_VERB_AGREEMENT", "Use a plural verb with this subject.", []string{group(1) + " " + verb}, "GRAMMAR", "grammar")
	})
	check.each(scan, `(?i)\b(a)`+space+`+([aeiou][A-Za-z'-]*)\b`, func(match []int, group func(int) string) {
		check.add(scan, match[0], match[1], "A_VOWEL_SOUND", "Use “an” before this vowel sound.", []string{"an " + group(2)}, "GRAMMAR", "grammar")
	})
	check.each(scan, `(?i)\b(an)`+space+`+([bcdfgjklmnpqrstvwxyz][A-Za-z'-]*)\b`, func(match []int, group func(int) string) {
		check.add(scan, match[0], match[1], "AN_CONSONANT_SOUND", "Use “a” before this consonant sound.", []string{"a " + group(2)}, "GRAMMAR", "grammar")
	})
	check.each(scan, `(^|[.!?]`+space+`+)([a-z])`, func(match []int, group func(int) string) {
		check.add(scan, match[4], match[4]+1, "SENTENCE_CAPITALIZATION", "Start this sentence with a capital letter.", []string{strings.ToUpper(group(2))}, "CASING", "grammar")
	})
}

// each calls found with the byte offsets of every match of pattern, in
// order, as String.prototype.matchAll did.
func (GrammarCheck) each(scan *GrammarScan, pattern string, found func(match []int, group func(int) string)) {
	for _, match := range regexp.MustCompile(pattern).FindAllStringSubmatchIndex(scan.text, -1) {
		group := func(index int) string {
			if match[2*index] < 0 {
				return ""
			}
			return scan.text[match[2*index]:match[2*index+1]]
		}
		found(match, group)
	}
}

// repeatedWords reports the second of two equal words (ignoring case)
// separated by white space: /\b([A-Za-z][A-Za-z'-]*)\s+\1\b/gi.
func (check GrammarCheck) repeatedWords(scan *GrammarScan) {
	text := scan.text
	for start := 0; start < len(text); {
		end, word := check.repeatAt(text, start)
		if end < 0 {
			start++
			continue
		}
		second := start + strings.LastIndex(text[start:end], word)
		check.add(scan, second, second+len(word), "REPEATED_WORD", "This word is repeated.", []string{}, "STYLE", "style")
		start = end
	}
}

// repeatAt answers the end of a repeated word starting at start and the
// first word, or -1 when none starts there.
func (check GrammarCheck) repeatAt(text string, start int) (int, string) {
	if !check.letter(text[start]) || (start > 0 && check.wordByte(text[start-1])) {
		return -1, ""
	}
	end := start + 1
	for end < len(text) && (check.letter(text[end]) || text[end] == '\'' || text[end] == '-') {
		end++
	}
	word := text[start:end]
	spaces := end
	for spaces < len(text) {
		character, size := utf8.DecodeRuneInString(text[spaces:])
		if !check.whiteSpace(character) {
			break
		}
		spaces += size
	}
	if spaces == end || len(text)-spaces < len(word) || !strings.EqualFold(text[spaces:spaces+len(word)], word) {
		return -1, ""
	}
	after := spaces + len(word)
	if check.wordByte(text[after-1]) == (after < len(text) && check.wordByte(text[after])) {
		return -1, ""
	}
	return after, word
}

// lowercaseI reports a standalone lower-case "i":
// /(^|[^A-Za-z])i(?=$|[^A-Za-z])/g.
func (check GrammarCheck) lowercaseI(scan *GrammarScan) {
	text := scan.text
	for index := 0; index < len(text); index++ {
		if text[index] != 'i' || (index > 0 && check.letter(text[index-1])) || (index+1 < len(text) && check.letter(text[index+1])) {
			continue
		}
		check.add(scan, index, index+1, "LOWERCASE_I", "The pronoun “I” should be capitalized.", []string{"I"}, "CASING", "grammar")
	}
}

// add records an issue between two byte offsets of the text.
func (check GrammarCheck) add(scan *GrammarScan, from int, to int, ruleId string, message string, replacements []string, category string, issueType string) {
	offset, length := scan.units[from], scan.units[to]-scan.units[from]
	if scan.disabled[ruleId] || length < 1 || offset < 0 || offset+length > scan.length {
		return
	}
	shortMessage := "Grammar"
	switch category {
	case "STYLE":
		shortMessage = "Style"
	case "PUNCTUATION":
		shortMessage = "Punctuation"
	}
	unique := []string{}
	for _, replacement := range replacements {
		if replacement != "" && !(TextValue{}).Includes(unique, replacement) && len(unique) < 5 {
			unique = append(unique, replacement)
		}
	}
	scan.matches = append(scan.matches, GrammarMatch{
		Offset: offset, Length: length, Message: message, ShortMessage: shortMessage,
		Replacements: unique, RuleId: ruleId, RuleDescription: message,
		Category: category, CategoryName: category[:1] + strings.ToLower(category[1:]),
		IssueType: issueType, Context: check.context(scan.text, offset, length),
	})
}

// nonOverlapping sorts the issues and keeps each one that starts after the
// previous issue in the sorted list ends, at most 100.
func (GrammarCheck) nonOverlapping(matches []GrammarMatch) []GrammarMatch {
	sort.SliceStable(matches, func(left, right int) bool {
		a, b := matches[left], matches[right]
		if a.Offset != b.Offset {
			return a.Offset < b.Offset
		}
		if a.Length != b.Length {
			return a.Length > b.Length
		}
		return TextOrder{}.Compare(a.RuleId, b.RuleId) < 0
	})
	kept := []GrammarMatch{}
	for index, match := range matches {
		if (index == 0 || match.Offset >= matches[index-1].Offset+matches[index-1].Length) && len(kept) < 100 {
			kept = append(kept, match)
		}
	}
	return kept
}

func (GrammarCheck) context(text string, offset int, length int) GrammarContext {
	units := utf16.Encode([]rune(text))
	start := max(0, offset-30)
	end := min(len(units), offset+length+30)
	return GrammarContext{Text: string(utf16.Decode(units[start:end])), Offset: offset - start, Length: length}
}

func (GrammarCheck) detectLanguage(text string, requested string) string {
	if requested != "auto" {
		return requested
	}
	letters, latin := 0, 0
	for _, character := range text {
		if unicode.IsLetter(character) {
			letters++
			if character < 0x80 {
				latin++
			}
		}
	}
	if letters == 0 || float64(latin)/float64(letters) >= 0.8 {
		return "en-US"
	}
	return "und"
}

func (GrammarCheck) preserveCase(source string, replacement string) string {
	first := source[:1]
	if first == strings.ToUpper(first) {
		return strings.ToUpper(replacement[:1]) + replacement[1:]
	}
	return replacement
}

// unitOffsets answers, for every byte offset of text, how many UTF-16 code
// units precede it.
func (GrammarCheck) unitOffsets(text string) []int {
	units := make([]int, len(text)+1)
	count := 0
	for index, character := range text {
		for byteIndex := index; byteIndex < index+utf8.RuneLen(character) && byteIndex < len(text); byteIndex++ {
			units[byteIndex] = count
		}
		count += utf16.RuneLen(character)
	}
	units[len(text)] = count
	return units
}

func (check GrammarCheck) whiteSpace(character rune) bool {
	return regexp.MustCompile(check.space()).MatchString(string(character))
}

// space is JavaScript's \s.
func (GrammarCheck) space() string {
	return `[\t\n\x0B\f\r \x{00A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}\x{FEFF}]`
}

func (GrammarCheck) letter(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
}

func (check GrammarCheck) wordByte(character byte) bool {
	return check.letter(character) || (character >= '0' && character <= '9') || character == '_'
}

func (GrammarCheck) ruleId() *regexp.Regexp { return regexp.MustCompile(`^[A-Z0-9_-]{1,64}$`) }
