package devicebox

type waitingPatch struct {
	Waiting map[string]bool `json:"waiting"`
}
