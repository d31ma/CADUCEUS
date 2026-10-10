// GrammarCheckService runs the bounded, self-hosted grammar check; nothing
// is stored and no other service is contacted.
// @Service
type GrammarCheckService struct{}

// Check answers the issues of text in the language, skipping the disabled
// rules.
func (service *GrammarCheckService) Check(text string, language string, disabledRules []string) GrammarResult {
	return GrammarCheck{}.Check(text, language, disabledRules)
}
