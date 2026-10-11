// ContactLink is one labelled value of the contact details; Href is empty for
// a value that is not a link.
type ContactLink struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Href  string `json:"href"`
}
