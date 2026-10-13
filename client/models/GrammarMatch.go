type GrammarMatch struct {
	Offset       float64  `json:"offset"`
	Length       float64  `json:"length"`
	Message      string   `json:"message"`
	ShortMessage string   `json:"shortMessage"`
	Category     string   `json:"category"`
	RuleID       string   `json:"ruleId"`
	Replacements []string `json:"replacements"`
}
