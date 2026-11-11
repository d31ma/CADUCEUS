import "time"

type JobBudget struct {
	Writes   int       `json:"writes"`
	Deadline time.Time `json:"deadline"`
}
