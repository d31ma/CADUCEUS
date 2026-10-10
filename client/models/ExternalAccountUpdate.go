type ExternalAccountUpdate struct {
	Action   string `json:"action"`
	Revision int    `json:"revision"`
	Color    string `json:"color,omitempty"`
	Order    *int   `json:"order,omitempty"`
}
