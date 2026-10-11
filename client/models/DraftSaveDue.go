// The editor body when its autosave timer fires. TooLarge is set when the
// body cannot travel in one event (64 KiB).
type DraftSaveDue struct {
	HTML     string `json:"html"`
	Text     string `json:"text"`
	TooLarge bool   `json:"tooLarge"`
}
