type DomainRow struct {
	DomainId    string    `json:"domainId"`
	Name        string    `json:"name"`
	RowClass    string    `json:"rowClass"`
	StateText   string    `json:"stateText"`
	StateClass  string    `json:"stateClass"`
	StateIcon   string    `json:"stateIcon"`
	StateNote   string    `json:"stateNote"`
	HasNote     bool      `json:"hasNote"`
	Marks       []DnsMark `json:"marks"`
	MarksLabel  string    `json:"marksLabel"`
	ActionText  string    `json:"actionText"`
	ActionClass string    `json:"actionClass"`
	ActionLabel string    `json:"actionLabel"`
}
