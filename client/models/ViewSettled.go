// ViewSettled is published at the end of an OnEvent listener that waited for
// the server: the page renders again when an event is delivered, and not when
// such a listener returns.
type ViewSettled struct {
	Source string `json:"source"`
}
