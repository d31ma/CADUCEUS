// The server draft this tab is writing, kept in sessionStorage as
// caduceus_compose_draft so a reload resumes it.
type DraftPointer struct {
	DraftID  string `json:"draftId"`
	ClientID string `json:"clientId"`
	Version  int    `json:"version"`
}
