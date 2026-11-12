import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// @Service
type SidebarRowService struct{}

func (service *SidebarRowService) Rows(summaries []ConversationSummary, channels []Channel, currentId string) []ConversationRow {
	return service.rowsOf(summaries, channels, currentId)
}

func (service *SidebarRowService) Section(rows []ConversationRow, section string) []ConversationRow {
	chosen := service.inSection(rows, section)
	slices.SortStableFunc(chosen, service.orderFor(section))
	return chosen
}

func (service *SidebarRowService) rowsOf(summaries []ConversationSummary, channels []Channel, currentId string) []ConversationRow {
	rows := []ConversationRow{}
	for _, summary := range summaries {
		rows = append(rows, service.row(summary, service.channelFor(summary.ConversationId, channels), currentId))
	}
	return rows
}

func (service *SidebarRowService) row(summary ConversationSummary, channel Channel, currentId string) ConversationRow {
	current := summary.ConversationId == currentId
	return ConversationRow{
		ConversationId:  summary.ConversationId,
		Title:           summary.Title,
		Kind:            summary.Kind,
		Section:         service.sectionOf(summary),
		Initials:        service.initialsOf(summary),
		UnreadCount:     summary.UnreadCount,
		MentionCount:    summary.MentionCount,
		Unread:          summary.UnreadCount > 0,
		CountLabel:      service.countLabel(summary.UnreadCount, summary.MentionCount),
		Private:         channel.Visibility == service.visibilityPrivate(),
		HasEmailAddress: channel.EmailAddress != "",
		Archived:        channel.Archived,
		Current:         current,
		AriaCurrent:     service.AriaCurrent(current),
		LastMessageAt:   summary.LastMessageAt,
	}
}

func (service *SidebarRowService) AriaCurrent(current bool) *string {
	return service.pageWhen(current)
}

func (service *SidebarRowService) pageWhen(current bool) *string {
	if !current {
		return nil
	}
	page := "page"
	return &page
}

func (service *SidebarRowService) channelFor(conversationId string, channels []Channel) Channel {
	found := Channel{}
	for _, channel := range channels {
		found = service.matching(found, channel, conversationId)
	}
	return found
}

func (service *SidebarRowService) matching(found Channel, channel Channel, conversationId string) Channel {
	if channel.ConversationId == conversationId {
		return channel
	}
	return found
}

func (service *SidebarRowService) sectionOf(summary ConversationSummary) string {
	switch {
	case summary.Kind == service.kindChannel():
		return service.sectionChannels()
	case summary.HasExternal:
		return service.sectionEmail()
	}
	return service.sectionDirect()
}

func (service *SidebarRowService) initialsOf(summary ConversationSummary) string {
	isChannel := summary.Kind == service.kindChannel()
	return service.twoLetters(service.pick(isChannel, summary.Title, service.personLetters(summary.Title)))
}

func (service *SidebarRowService) personLetters(title string) string {
	letters := ""
	for _, word := range strings.Fields(title) {
		letters += string([]rune(word)[0])
	}
	return letters + string([]rune(title + service.space())[1:])
}

func (service *SidebarRowService) space() string {
	return " "
}

func (service *SidebarRowService) twoLetters(text string) string {
	letters := []rune(strings.ToUpper(strings.Join(strings.Fields(text), "")) + "  ")
	return strings.TrimSpace(string(letters[:service.initialsLength()]))
}

func (service *SidebarRowService) countLabel(unreadCount int, mentionCount int) string {
	parts := []string{service.unreadText(unreadCount), service.mentionText(mentionCount)}
	return strings.Join(slices.DeleteFunc(parts, service.isEmpty), ", ")
}

func (service *SidebarRowService) unreadText(count int) string {
	return service.pick(count > 0, strconv.Itoa(count)+" unread", "")
}

func (service *SidebarRowService) mentionText(count int) string {
	return service.pick(count > 0, strconv.Itoa(count)+service.space()+service.mentionWord(count), "")
}

func (service *SidebarRowService) mentionWord(count int) string {
	if count == 1 {
		return "mention"
	}
	return "mentions"
}

func (service *SidebarRowService) isEmpty(text string) bool {
	return text == ""
}

func (service *SidebarRowService) pick(first bool, chosen string, otherwise string) string {
	if first {
		return chosen
	}
	return otherwise
}

func (service *SidebarRowService) inSection(rows []ConversationRow, section string) []ConversationRow {
	return slices.DeleteFunc(append([]ConversationRow{}, rows...), func(row ConversationRow) bool {
		return row.Section != section
	})
}

func (service *SidebarRowService) orderFor(section string) func(ConversationRow, ConversationRow) int {
	if section == service.sectionChannels() {
		return service.byTitle
	}
	return service.newestFirst
}

func (service *SidebarRowService) byTitle(left ConversationRow, right ConversationRow) int {
	return cmp.Or(
		strings.Compare(strings.ToLower(left.Title), strings.ToLower(right.Title)),
		strings.Compare(left.ConversationId, right.ConversationId),
	)
}

func (service *SidebarRowService) newestFirst(left ConversationRow, right ConversationRow) int {
	return cmp.Or(strings.Compare(right.LastMessageAt, left.LastMessageAt), service.byTitle(left, right))
}

func (service *SidebarRowService) kindChannel() string {
	return "channel"
}

func (service *SidebarRowService) visibilityPrivate() string {
	return "private"
}

func (service *SidebarRowService) sectionChannels() string {
	return "channels"
}

func (service *SidebarRowService) sectionDirect() string {
	return "direct"
}

func (service *SidebarRowService) sectionEmail() string {
	return "email"
}

func (service *SidebarRowService) initialsLength() int {
	return 2
}
