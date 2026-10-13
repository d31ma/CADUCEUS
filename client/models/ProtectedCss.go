type ProtectedCss struct {
	Css           string   `json:"css"`
	RemoteCount   int      `json:"remoteCount"`
	BlockedCount  int      `json:"blockedCount"`
	TrackingCount int      `json:"trackingCount"`
	RemoteUrls    []string `json:"remoteUrls"`
}
