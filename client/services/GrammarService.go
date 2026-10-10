import (
	"slices"
	"strconv"
	"strings"
)

// GrammarService runs the opt-in local grammar check of the composer: its
// preferences, the /grammar/check request, the issue list and the status
// line. The editor service sends the plain text after a 700 ms debounce and
// paints the highlights, because ranges and the CSS Highlight API are
// browser-only.
//
// @Service
type GrammarService struct {
	// @Needs
	grammarClient *GrammarClient
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher

	Enabled  bool
	Language string
	Issues   []GrammarIssue
	Checking bool
	Status   string

	token         string
	disabledRules []string
	text          string
	request       int
}

// @OnStart
func (grammar *GrammarService) Start() {
	grammar.Language = grammar.automaticLanguage()
	grammar.Status = grammar.offStatus()
	grammar.Issues = []GrammarIssue{}
	grammar.disabledRules = []string{}
}

// SessionChanged keeps the signed-in mailbox and loads its grammar choices.
//
// @OnEvent({"replay":true})
func (grammar *GrammarService) SessionChanged(changed SessionChanged) {
	grammar.token = changed.Token
	preferences, problem := grammar.usersClient.Preferences(grammar.authorization())
	grammar.keepPreferences(preferences, grammar.succeeded(problem))
	grammar.publish(GrammarTextWanted{Reason: "start"})
}

// GrammarTextChanged checks the editor text when the check is on.
//
// @OnEvent
func (grammar *GrammarService) GrammarTextChanged(changed GrammarTextChanged) {
	grammar.text = changed.Text
	grammar.request++
	grammar.checkWhen(grammar.wanted(grammar.Enabled, changed.Text), changed.Text, grammar.request)
	grammar.publish(ComposeChanged{Reason: grammar.changedReason()})
}

// Toggle turns the local grammar check on or off and saves the choice.
func (grammar *GrammarService) Toggle() {
	enabled := !grammar.Enabled
	grammar.switchTo(enabled, grammar.saved(map[string]any{"grammarCheckEnabled": enabled}))
}

// ChangeLanguage chooses automatic English, US English or British English.
func (grammar *GrammarService) ChangeLanguage(language string) {
	grammar.useLanguageWhen(grammar.known(language), language)
}

// Focus selects the text of one issue in the editor.
func (grammar *GrammarService) Focus(index int) {
	issue := grammar.issueAt(index)
	grammar.publish(GrammarFocusRequested{Offset: issue.Offset, Length: issue.Length})
}

// Replace applies one suggested correction in the editor.
func (grammar *GrammarService) Replace(index int, replacement string) {
	issue := grammar.issueAt(index)
	grammar.publish(GrammarReplacementRequested{Offset: issue.Offset, Length: issue.Length, Replacement: replacement})
}

// Ignore stops reporting a rule for this mailbox.
func (grammar *GrammarService) Ignore(rule string) {
	grammar.ignoreWhen(grammar.ignorable(rule), rule)
}

// Title is the label of the toolbar toggle.
func (grammar *GrammarService) Title() string {
	return grammar.titleOf(grammar.Enabled, grammar.Checking, len(grammar.Issues))
}

func (grammar *GrammarService) authorization() string {
	return "Bearer " + grammar.token
}

func (grammar *GrammarService) publish(value any) {
	grammar.applicationEventPublisher.PublishEvent(value, false)
}

func (GrammarService) automaticLanguage() string { return "auto" }

func (GrammarService) offStatus() string { return "Grammar check is off." }

func (GrammarService) waitingStatus() string { return "Waiting for text…" }

func (GrammarService) changedReason() string { return "changed" }

func (GrammarService) languages() []string { return []string{"auto", "en-US", "en-GB"} }

func (GrammarService) succeeded(problem error) []bool {
	if problem == nil {
		return []bool{true}
	}
	return nil
}

func (grammar *GrammarService) keepPreferences(preferences UserPreferences, succeeded []bool) {
	for range succeeded {
		grammar.applyPreferences(preferences)
	}
}

func (grammar *GrammarService) checkWhen(wanted []bool, text string, request int) {
	for range wanted {
		grammar.check(text, request)
	}
}

func (grammar *GrammarService) useLanguageWhen(known []bool, language string) {
	for range known {
		grammar.useLanguage(language)
	}
}

func (grammar *GrammarService) ignoreWhen(ignorable []bool, rule string) {
	for range ignorable {
		grammar.ignoreRule(rule)
	}
}

func (grammar *GrammarService) applyPreferences(preferences UserPreferences) {
	grammar.Enabled = preferences.GrammarCheckEnabled
	grammar.Language = grammar.knownLanguage(preferences.GrammarCheckLanguage)
	grammar.disabledRules = GrammarUtils{}.ValidRules(preferences.GrammarCheckDisabledRules)
	grammar.Status = grammar.idleStatus(grammar.Enabled)
}

func (grammar *GrammarService) knownLanguage(language string) string {
	if slices.Contains(grammar.languages(), language) {
		return language
	}
	return grammar.automaticLanguage()
}

func (grammar *GrammarService) idleStatus(enabled bool) string {
	if enabled {
		return grammar.waitingStatus()
	}
	return grammar.offStatus()
}

func (grammar *GrammarService) saved(patch map[string]any) bool {
	_, problem := grammar.usersClient.UpdatePreferences(patch, grammar.authorization())
	return problem == nil
}

func (grammar *GrammarService) switchTo(enabled bool, saved bool) {
	switch {
	case !saved:
		grammar.Status = "Could not save grammar preference."
	case enabled:
		grammar.Enabled = true
		grammar.Status = grammar.waitingStatus()
		grammar.publish(GrammarTextWanted{Reason: "enabled"})
	default:
		grammar.Enabled = false
		grammar.request++
		grammar.Issues = []GrammarIssue{}
		grammar.Status = grammar.offStatus()
		grammar.publish(GrammarHighlightsChanged{Ranges: []GrammarRange{}})
	}
}

func (grammar *GrammarService) known(language string) []bool {
	if slices.Contains(grammar.languages(), language) {
		return []bool{true}
	}
	return nil
}

func (grammar *GrammarService) useLanguage(language string) {
	previous := grammar.Language
	grammar.Language = language
	grammar.Language = grammar.languageAfterSave(previous, language, grammar.saved(map[string]any{"grammarCheckLanguage": language}))
	grammar.publish(GrammarTextWanted{Reason: "language"})
}

func (GrammarService) languageAfterSave(previous string, language string, saved bool) string {
	if saved {
		return language
	}
	return previous
}

func (grammar *GrammarService) wanted(enabled bool, text string) []bool {
	if !enabled {
		return nil
	} else if strings.TrimSpace(text) == "" {
		grammar.showIssues(nil, "No text to check.")
		return nil
	}
	return []bool{true}
}

func (grammar *GrammarService) check(text string, request int) {
	grammar.Checking = true
	grammar.Status = "Checking locally…"
	answer, problem := grammar.grammarClient.Check(GrammarInput{Text: text, Language: grammar.Language, DisabledRules: grammar.disabledRules}, grammar.authorization())
	for range grammar.current(request, text) {
		grammar.answered(answer, problem)
	}
}

func (grammar *GrammarService) current(request int, text string) []bool {
	if request == grammar.request && text == grammar.text {
		return []bool{true}
	}
	return nil
}

func (grammar *GrammarService) answered(answer GrammarAnswer, problem error) {
	issues := GrammarUtils{}.Issues(answer.Matches, grammar.text)
	grammar.Checking = false
	grammar.showIssues(grammar.issuesOr(issues, problem), grammar.statusOf(issues, answer.DetectedLanguage, problem))
}

func (GrammarService) issuesOr(issues []GrammarIssue, problem error) []GrammarIssue {
	if problem != nil {
		return nil
	}
	return issues
}

func (GrammarService) statusOf(issues []GrammarIssue, detectedLanguage string, problem error) string {
	if problem != nil {
		return "Grammar check unavailable."
	}
	return GrammarUtils{}.Status(len(issues), detectedLanguage)
}

func (grammar *GrammarService) showIssues(issues []GrammarIssue, status string) {
	grammar.Issues = grammar.numbered(issues)
	grammar.Status = status
	grammar.publish(GrammarHighlightsChanged{Ranges: grammar.ranges()})
}

func (grammar *GrammarService) numbered(issues []GrammarIssue) []GrammarIssue {
	numbered := []GrammarIssue{}
	for index, issue := range issues {
		issue.Index = index
		numbered = append(numbered, issue)
	}
	return numbered
}

func (grammar *GrammarService) ranges() []GrammarRange {
	ranges := []GrammarRange{}
	for _, issue := range grammar.Issues {
		ranges = append(ranges, GrammarRange{Offset: issue.Offset, Length: issue.Length})
	}
	return ranges
}

func (grammar *GrammarService) issueAt(index int) GrammarIssue {
	if index >= 0 && index < len(grammar.Issues) {
		return grammar.Issues[index]
	}
	return GrammarIssue{}
}

func (grammar *GrammarService) ignorable(rule string) []bool {
	valid := GrammarUtils{}.ValidRule(rule)
	if valid && !slices.Contains(grammar.disabledRules, rule) {
		return []bool{true}
	}
	return nil
}

func (grammar *GrammarService) ignoreRule(rule string) {
	rules := append(append([]string{}, grammar.disabledRules...), rule)
	for range grammar.succeeded(grammar.savedProblem(map[string]any{"grammarCheckDisabledRules": rules})) {
		grammar.keepIgnored(rules, rule)
	}
}

func (grammar *GrammarService) savedProblem(patch map[string]any) error {
	_, problem := grammar.usersClient.UpdatePreferences(patch, grammar.authorization())
	return problem
}

func (grammar *GrammarService) keepIgnored(rules []string, rule string) {
	grammar.disabledRules = rules
	remaining := grammar.issuesWithout(rule)
	grammar.showIssues(remaining, GrammarUtils{}.IssueCount(len(remaining)))
}

func (grammar *GrammarService) issuesWithout(rule string) []GrammarIssue {
	kept := []GrammarIssue{}
	for _, issue := range grammar.Issues {
		kept = append(kept, grammar.unlessRule(issue, rule)...)
	}
	return kept
}

func (GrammarService) unlessRule(issue GrammarIssue, rule string) []GrammarIssue {
	if issue.RuleID == rule {
		return nil
	}
	return []GrammarIssue{issue}
}

func (GrammarService) titleOf(enabled bool, checking bool, count int) string {
	switch {
	case !enabled:
		return "Enable grammar check"
	case checking:
		return "Grammar check: checking"
	}
	return "Grammar check: " + strconv.Itoa(count) + " issues"
}
