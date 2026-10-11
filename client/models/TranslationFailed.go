// TranslationFailed says the on-device translation of a message failed.
type TranslationFailed struct {
	MessageId string `json:"messageId"`
	Message   string `json:"message"`
}
