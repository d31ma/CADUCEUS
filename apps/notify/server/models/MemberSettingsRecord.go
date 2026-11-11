type MemberSettingsRecord struct {
	Id                       string   `json:"id,omitempty"`
	DomainId                 string   `json:"domainId"`
	MemberId                 string   `json:"memberId"`
	EmailSummaryAfterMinutes int      `json:"emailSummaryAfterMinutes"`
	MutedConversationIds     []string `json:"mutedConversationIds"`
	DisplayName              string   `json:"displayName"`
	PrimaryAddress           string   `json:"primaryAddress"`
	State                    string   `json:"state"`
}
