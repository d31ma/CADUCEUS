// The fields of one message that a mail action changed; a field left out is unchanged.
type EmailChange struct {
	ID           string    `json:"id"`
	Domain       *string   `json:"domain"`
	Folder       *string   `json:"folder"`
	SystemFolder *string   `json:"systemFolder"`
	Labels       *[]string `json:"labels"`
	Read         *bool     `json:"read"`
	Starred      *bool     `json:"starred"`
}
