import "slices"

// @Service
type NotificationPlaceService struct {
	// @Needs
	sidebar *SidebarService
	// @Needs
	kinds *NotificationKindService
}

func (places *NotificationPlaceService) Name(conversationId string) string {
	return places.nameOf(places.rows()[conversationId])
}

func (places *NotificationPlaceService) Initials(conversationId string) string {
	return places.rows()[conversationId].Initials
}

func (places *NotificationPlaceService) Word(kind string) string {
	return places.kinds.PlaceWord(kind)
}

func (places *NotificationPlaceService) rows() map[string]ConversationRow {
	rows := map[string]ConversationRow{}
	for _, row := range slices.Concat(places.sidebar.Channels, places.sidebar.Directs, places.sidebar.Emails) {
		rows[row.ConversationId] = row
	}
	return rows
}

func (places *NotificationPlaceService) nameOf(row ConversationRow) string {
	if row.Kind != places.channelKind() {
		return row.Title
	}
	return places.channelMark() + row.Title
}

func (places *NotificationPlaceService) channelKind() string {
	return "channel"
}

func (places *NotificationPlaceService) channelMark() string {
	return "#"
}
