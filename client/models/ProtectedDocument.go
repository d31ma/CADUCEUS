type ProtectedDocument struct {
	DocumentHtml        string   `json:"documentHtml"`
	Html                string   `json:"html"`
	RemoteCount         int      `json:"remoteCount"`
	BlockedCount        int      `json:"blockedCount"`
	TrackingCount       int      `json:"trackingCount"`
	QuotedCount         int      `json:"quotedCount"`
	SuspiciousLinkCount int      `json:"suspiciousLinkCount"`
	RemoteUrls          []string `json:"remoteUrls"`
}
