type Recipient struct {
	Address     string   `json:"address"`
	DisplayName string   `json:"displayName"`
	FirstName   string   `json:"firstName"`
	Initials    string   `json:"initials"`
	Kind        string   `json:"kind"`
	Role        string   `json:"role"`
	Delivery    string   `json:"delivery"`
	State       string   `json:"state"`
	Label       string   `json:"label"`
	AriaLabel   string   `json:"ariaLabel"`
	Allowed     []string `json:"allowed"`
	ChatAllowed bool     `json:"chatAllowed"`
	ChatReason  string   `json:"chatReason"`
	BothReason  string   `json:"bothReason"`
}
