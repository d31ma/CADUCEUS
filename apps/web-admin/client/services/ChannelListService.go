import (
	"slices"
	"strconv"
	"strings"
)

// @Service
type ChannelListService struct {
	Domain         string
	Search         string
	Show           string
	State          string
	Rows           []ChannelRow
	Heading        string
	Summary        string
	Showing        string
	NoResultsTitle string
	LoadProblem    string
	channels       []Channel
	// @Needs
	session *AdminSessionService
	// @Needs
	channelsClient *ChannelsClient
	// @Needs
	problems                  *AdminProblemService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnEvent({"condition":"value.view == 'channels'","replay":true})
// @InBackground
func (list *ChannelListService) Opened(opened AdminViewOpened) {
	list.Domain = opened.DomainId
	list.Search = ""
	list.Show = list.allChannels()
	list.load()
	list.applicationEventPublisher.PublishEvent(AdminViewLoaded{View: opened.View, Message: list.Heading}, false)
}

func (list *ChannelListService) Refresh() {
	list.load()
}

func (list *ChannelListService) SearchWith(text string) {
	list.Search = text
	list.show()
}

func (list *ChannelListService) ShowOnly(show string) {
	list.Show = show
	list.show()
}

func (list *ChannelListService) ClearFilters() {
	list.Search = ""
	list.Show = list.allChannels()
	list.show()
}

func (list *ChannelListService) Find(channelId string) Channel {
	return list.channelsById()[channelId]
}

func (list *ChannelListService) Replace(channel Channel) {
	channels := list.channelsById()
	channels[channel.ChannelId] = channel
	list.channels = list.inOrder(channels)
	list.show()
}

func (list *ChannelListService) load() {
	list.State = list.loading()
	channels, failure := list.channelsClient.Channels(list.session.Authorization())
	list.channels = channels
	list.show()
	problem := list.problems.Of(failure)
	list.LoadProblem = list.loadProblemText(problem)
	list.State = list.failedWhen(problem, list.State)
}

func (list *ChannelListService) show() {
	visible := list.visible()
	list.Rows = list.rowsOf(visible)
	list.State = list.emptyWhen(len(list.channels), list.resultsState(len(visible)))
	list.Heading = list.countText(len(list.channels))
	list.Summary = list.count(list.withoutAddress) + " with an email address · " + list.count(list.active) + " archived"
	list.Showing = "Showing " + strconv.Itoa(len(visible)) + " of " + strconv.Itoa(len(list.channels)) + "."
	list.NoResultsTitle = list.noResultsText(strings.TrimSpace(list.Search))
}

func (list *ChannelListService) visible() []Channel {
	shown := slices.DeleteFunc(slices.Clone(list.channels), list.hiddenByShow)
	return slices.DeleteFunc(shown, list.hiddenBySearch)
}

func (list *ChannelListService) hiddenByShow(channel Channel) bool {
	return !list.shownBy(channel)[list.Show]
}

func (list *ChannelListService) shownBy(channel Channel) map[string]bool {
	return map[string]bool{
		list.allChannels():       true,
		list.publicVisibility():  channel.Visibility == list.publicVisibility(),
		list.privateVisibility(): channel.Visibility == list.privateVisibility(),
		"email":                  channel.EmailAddress != "",
		"archived":               channel.Archived,
	}
}

func (list *ChannelListService) hiddenBySearch(channel Channel) bool {
	return !strings.Contains(strings.ToLower(channel.Name+"\n"+channel.EmailAddress), list.searchTerm())
}

func (list *ChannelListService) searchTerm() string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(list.Search)), list.channelSign())
}

func (list *ChannelListService) count(skipped func(Channel) bool) string {
	return strconv.Itoa(len(slices.DeleteFunc(slices.Clone(list.channels), skipped)))
}

func (list *ChannelListService) withoutAddress(channel Channel) bool {
	return channel.EmailAddress == ""
}

func (list *ChannelListService) active(channel Channel) bool {
	return !channel.Archived
}

func (list *ChannelListService) rowsOf(channels []Channel) []ChannelRow {
	rows := []ChannelRow{}
	for _, channel := range channels {
		rows = append(rows, list.rowOf(channel))
	}
	return rows
}

func (list *ChannelListService) rowOf(channel Channel) ChannelRow {
	label := list.channelSign() + channel.Name
	hasAddress := channel.EmailAddress != ""
	action := map[bool]string{true: "Edit address", false: "Add address"}[hasAddress]
	return ChannelRow{
		ChannelId:      channel.ChannelId,
		Name:           channel.Name,
		Topic:          channel.Topic,
		NameIcon:       list.nameIcons()[channel.Visibility],
		VisibilityText: list.visibilityTexts()[channel.Visibility],
		ChipVariant:    list.chipVariants()[channel.Visibility],
		VisibilityIcon: list.visibilityIcons()[channel.Visibility],
		HasAddress:     hasAddress,
		EmailAddress:   channel.EmailAddress,
		AddressAction:  action,
		AddressLabel:   action + " for " + label,
		Archived:       channel.Archived,
		ArchiveLabel:   "Archive " + label,
		RestoreLabel:   "Unarchive " + label,
		RowClass:       map[bool]string{true: "is-muted", false: ""}[channel.Archived],
	}
}

func (list *ChannelListService) nameIcons() map[string]string {
	return map[string]string{list.publicVisibility(): "hash", list.privateVisibility(): list.lockIcon()}
}

func (list *ChannelListService) visibilityTexts() map[string]string {
	return map[string]string{list.publicVisibility(): "Public", list.privateVisibility(): "Private"}
}

func (list *ChannelListService) chipVariants() map[string]string {
	return map[string]string{list.publicVisibility(): "outlined", list.privateVisibility(): "tonal"}
}

func (list *ChannelListService) visibilityIcons() map[string]string {
	return map[string]string{list.publicVisibility(): "globe", list.privateVisibility(): list.lockIcon()}
}

func (list *ChannelListService) channelsById() map[string]Channel {
	channels := map[string]Channel{}
	for _, channel := range list.channels {
		channels[channel.ChannelId] = channel
	}
	return channels
}

func (list *ChannelListService) inOrder(channels map[string]Channel) []Channel {
	ordered := []Channel{}
	for _, channel := range list.channels {
		ordered = append(ordered, channels[channel.ChannelId])
	}
	return ordered
}

func (list *ChannelListService) emptyWhen(total int, state string) string {
	if total == 0 {
		return "empty"
	}
	return state
}

func (list *ChannelListService) resultsState(shown int) string {
	if shown == 0 {
		return "noResults"
	}
	return "ready"
}

func (list *ChannelListService) failedWhen(problem string, state string) string {
	if problem == "" {
		return state
	}
	return "failed"
}

func (list *ChannelListService) countText(total int) string {
	if total == 1 {
		return "1 channel"
	}
	return strconv.Itoa(total) + " channels"
}

func (list *ChannelListService) noResultsText(search string) string {
	if search == "" {
		return "No channels match this filter"
	}
	return "No channels match “" + search + "”"
}

func (list *ChannelListService) loadProblemText(problem string) string {
	if problem == "signedOut" {
		return "Your session ended. Sign in again, then try again."
	}
	return "Mesail could not reach the server. Check your connection, then try again."
}

func (list *ChannelListService) loading() string {
	return "loading"
}

func (list *ChannelListService) allChannels() string {
	return "all"
}

func (list *ChannelListService) publicVisibility() string {
	return "public"
}

func (list *ChannelListService) privateVisibility() string {
	return "private"
}

func (list *ChannelListService) lockIcon() string {
	return "lock"
}

func (list *ChannelListService) channelSign() string {
	return "#"
}
