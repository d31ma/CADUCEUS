// ConfirmationGiven says the person accepted a confirm() question for one
// item, such as cancelling one scheduled send.
type ConfirmationGiven struct {
	Action string `json:"action"`
	ID     string `json:"id"`
}
