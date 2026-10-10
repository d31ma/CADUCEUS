// The server could not answer; search the offline index instead.
type OfflineSearchRequested struct {
	Request int    `json:"request"`
	Owner   string `json:"owner"`
	Query   string `json:"query"`
	Folder  string `json:"folder"`
	Domain  string `json:"domain"`
	Account string `json:"account"`
	Read    string `json:"read"`
	Starred string `json:"starred"`
	Limit   int    `json:"limit"`
}
