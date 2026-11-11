type InboundContent struct {
	Text        string              `json:"text"`
	BodyFormat  string              `json:"bodyFormat"`
	Blocks      []Block             `json:"blocks"`
	Truncated   bool                `json:"truncated"`
	Attachments []InboundAttachment `json:"attachments"`
}
