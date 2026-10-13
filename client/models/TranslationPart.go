type TranslationPart struct {
	MessageId string `json:"messageId"`
	Kind      string `json:"kind"`
	Index     int    `json:"index"`
	Count     int    `json:"count"`
	Text      string `json:"text"`
}
