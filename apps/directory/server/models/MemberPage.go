type MemberPage struct {
	Items      []MemberSummary `json:"items"`
	NextCursor string          `json:"nextCursor"`
}
