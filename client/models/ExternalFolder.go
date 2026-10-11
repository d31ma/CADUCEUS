type ExternalFolder struct {
	StableID       string        `json:"stableId"`
	Revision       string        `json:"revision"`
	Name           string        `json:"name"`
	Role           string        `json:"role"`
	Delimiter      string        `json:"delimiter"`
	Attributes     []string      `json:"attributes"`
	Folder         string        `json:"folder"`
	AccountID      string        `json:"accountId"`
	Selectable     bool          `json:"selectable"`
	DisplayName    string        `json:"displayName"`
	Depth          int           `json:"depth"`
	ParentStableID string        `json:"parentStableId"`
	HasChildren    bool          `json:"hasChildren"`
	RowStyle       string        `json:"rowStyle"`
	Href           string        `json:"href"`
	Open           bool          `json:"open"`
	Actions        FolderActions `json:"actions"`
	HasActions     bool          `json:"hasActions"`
}
