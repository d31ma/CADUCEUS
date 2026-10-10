type ActivityReport struct {
	Events      []ActivityEvent `json:"events"`
	Metrics     ActivityMetrics `json:"metrics"`
	GeneratedAt string          `json:"generatedAt"`
}
