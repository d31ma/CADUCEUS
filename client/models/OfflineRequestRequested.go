type OfflineRequestRequested struct {
	Id          string `json:"id"`
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	Method      string `json:"method"`
	Body        string `json:"body"`
	Message     string `json:"message"`
	CoalesceKey string `json:"coalesceKey"`
	EmailId     string `json:"emailId"`
}
