// Backspace or a touch swipe on a row asks to move messages to a folder.
type MailGesture struct {
	Kind    string `json:"kind"`
	EmailID string `json:"emailId"`
	Folder  string `json:"folder"`
}
