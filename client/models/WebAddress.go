type WebAddress struct {
	Valid          bool   `json:"valid"`
	Scheme         string `json:"scheme"`
	HasCredentials bool   `json:"hasCredentials"`
	Host           string `json:"host"`
	Port           string `json:"port"`
	Href           string `json:"href"`
}
