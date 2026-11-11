type ConversationRow struct {
	ConversationId  string `json:"conversationId"`
	Title           string `json:"title"`
	Kind            string `json:"kind"`
	Section         string `json:"section"`
	Initials        string `json:"initials"`
	UnreadCount     int    `json:"unreadCount"`
	MentionCount    int    `json:"mentionCount"`
	Unread          bool   `json:"unread"`
	CountLabel      string `json:"countLabel"`
	Private         bool   `json:"private"`
	HasEmailAddress bool   `json:"hasEmailAddress"`
	Archived        bool   `json:"archived"`
	Current         bool   `json:"current"`
	LastMessageAt   string `json:"lastMessageAt"`
}
