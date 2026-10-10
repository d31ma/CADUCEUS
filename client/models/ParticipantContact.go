type ParticipantContact struct {
	Name       string         `json:"name"`
	GivenName  string         `json:"givenName"`
	FamilyName string         `json:"familyName"`
	Title      string         `json:"title"`
	Company    string         `json:"company"`
	Notes      string         `json:"notes"`
	Phones     []ContactValue `json:"phones"`
	Urls       []ContactValue `json:"urls"`
	Emails     []ContactValue `json:"emails"`
}
