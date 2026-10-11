type RuleRow struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Mailbox         string `json:"mailbox"`
	Enabled         bool   `json:"enabled"`
	ConditionsLabel string `json:"conditionsLabel"`
	ActionsLabel    string `json:"actionsLabel"`
}
