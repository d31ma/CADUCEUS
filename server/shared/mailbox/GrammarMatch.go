// GrammarMatch is one reported issue.
type GrammarMatch struct {
	Offset          int            `json:"offset"`
	Length          int            `json:"length"`
	Message         string         `json:"message"`
	ShortMessage    string         `json:"shortMessage"`
	Replacements    []string       `json:"replacements"`
	RuleId          string         `json:"ruleId"`
	RuleDescription string         `json:"ruleDescription"`
	Category        string         `json:"category"`
	CategoryName    string         `json:"categoryName"`
	IssueType       string         `json:"issueType"`
	Context         GrammarContext `json:"context"`
}
