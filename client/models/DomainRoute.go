type DomainRoute struct {
	Match   string      `json:"match"`
	Enabled bool        `json:"enabled"`
	Action  RouteAction `json:"action"`
}
