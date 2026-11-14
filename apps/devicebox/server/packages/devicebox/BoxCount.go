package devicebox

type BoxCount struct {
	Id     string         `json:"id,omitempty"`
	Bucket string         `json:"bucket"`
	Counts map[string]int `json:"counts"`
}
