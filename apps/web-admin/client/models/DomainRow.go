type DomainRow struct {
	DomainId      string    `json:"domainId"`
	Name          string    `json:"name"`
	Tone          string    `json:"tone"`
	StateText     string    `json:"stateText"`
	StateColor    string    `json:"stateColor"`
	StateVariant  string    `json:"stateVariant"`
	StateIcon     string    `json:"stateIcon"`
	StateNote     string    `json:"stateNote"`
	HasNote       bool      `json:"hasNote"`
	Marks         []DnsMark `json:"marks"`
	MarksLabel    string    `json:"marksLabel"`
	ActionText    string    `json:"actionText"`
	ActionVariant string    `json:"actionVariant"`
	ActionLabel   string    `json:"actionLabel"`
}
