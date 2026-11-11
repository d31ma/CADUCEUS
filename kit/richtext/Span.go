package richtext

type Span struct {
	Text   string `json:"text"`
	Bold   bool   `json:"bold"`
	Italic bool   `json:"italic"`
	Code   bool   `json:"code"`
	Strike bool   `json:"strike"`
	Href   string `json:"href,omitempty"`
}
