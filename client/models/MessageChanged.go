type MessageChanged struct {
	EmailId      string   `json:"emailId"`
	Folder       string   `json:"folder"`
	SystemFolder string   `json:"systemFolder"`
	Labels       []string `json:"labels"`
	Read         bool     `json:"read"`
	Starred      bool     `json:"starred"`
}
