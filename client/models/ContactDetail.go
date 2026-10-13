// ContactDetail is the selected contact, computed for the detail panel.
type ContactDetail struct {
	ID             string        `json:"id"`
	Heading        string        `json:"heading"`
	Initials       string        `json:"initials"`
	SourceLabel    string        `json:"sourceLabel"`
	FullName       string        `json:"fullName"`
	Company        string        `json:"company"`
	JobTitle       string        `json:"jobTitle"`
	Birthday       string        `json:"birthday"`
	Emails         []ContactLink `json:"emails"`
	Phones         []ContactLink `json:"phones"`
	Urls           []ContactLink `json:"urls"`
	Addresses      []ContactLink `json:"addresses"`
	GroupNames     []string      `json:"groupNames"`
	Nicknames      string        `json:"nicknames"`
	Notes          string        `json:"notes"`
	ShowProfile    bool          `json:"showProfile"`
	ShowNotes      bool          `json:"showNotes"`
	ReadOnly       bool          `json:"readOnly"`
	CanMessage     bool          `json:"canMessage"`
	DeleteQuestion string        `json:"deleteQuestion"`
}
