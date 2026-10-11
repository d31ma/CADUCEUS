// The message the composer opened with (retained). The editor shows HTML;
// SenderService picks the From address and signature.
type ComposeLoaded struct {
	Reason      string              `json:"reason"`
	From        string              `json:"from"`
	HTML        string              `json:"html"`
	DraftID     string              `json:"draftId"`
	ScheduledID string              `json:"scheduledId"`
	Recovered   bool                `json:"recovered"`
	SignatureID string              `json:"signatureId"`
	Attachments []ComposeAttachment `json:"attachments"`
}
