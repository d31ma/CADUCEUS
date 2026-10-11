// ComposeService asks the editor for the body of a send. The editor checks
// template blanks first.
type SendRequested struct {
	Mode string `json:"mode"`
}
