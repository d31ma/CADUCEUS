package devicebox

type ExpireReport struct {
	Writes int  `json:"writes"`
	More   bool `json:"more"`
}
