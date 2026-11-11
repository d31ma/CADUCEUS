package richtext

type Result struct {
	Blocks    []Block `json:"blocks"`
	Text      string  `json:"text"`
	Truncated bool    `json:"truncated"`
}
