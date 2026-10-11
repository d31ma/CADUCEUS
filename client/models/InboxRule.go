type InboxRule struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Mailbox        string          `json:"mailbox"`
	Enabled        bool            `json:"enabled"`
	StopProcessing bool            `json:"stopProcessing"`
	ConditionMatch string          `json:"conditionMatch"`
	Conditions     []RuleCondition `json:"conditions"`
	Actions        []RuleAction    `json:"actions"`
}
