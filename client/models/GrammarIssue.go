type GrammarIssue struct {
	Index        int      `json:"index"`
	Offset       int      `json:"offset"`
	Length       int      `json:"length"`
	Excerpt      string   `json:"excerpt"`
	Message      string   `json:"message"`
	ShortMessage string   `json:"shortMessage"`
	Category     string   `json:"category"`
	RuleID       string   `json:"ruleId"`
	Replacements []string `json:"replacements"`
}
