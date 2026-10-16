// One Go-owned editor generation; metadata inputs share the document’s revision.
type ComposeEditorSnapshot struct {
	Proof       *QuoteProof         `json:"proof,omitempty"`
	Document    string              `json:"document"`
	Revision    int                 `json:"revision"`
	Body        string              `json:"body"`
	Quote       string              `json:"quote"`
	Attachments []ComposeAttachment `json:"attachments"`
}
