// @Service
type ChannelArchiveService struct {
	Open        bool
	ChannelId   string
	Title       string
	AddressNote string
	Busy        bool
	Problem     string
	Notice      string
	label       string
	// @Needs
	channelList *ChannelListService
	// @Needs
	session *AdminSessionService
	// @Needs
	channelsClient *ChannelsClient
	// @Needs
	problems *AdminProblemService
}

func (archive *ChannelArchiveService) Ask(channelId string) {
	channel := archive.choose(channelId)
	archive.Title = "Archive " + archive.label + "?"
	archive.AddressNote = archive.addressNoteOf(channel.EmailAddress)
	archive.Problem = ""
	archive.Open = true
}

func (archive *ChannelArchiveService) Cancel() {
	archive.Open = false
}

func (archive *ChannelArchiveService) Confirm() {
	archived := true
	archive.Problem = archive.update(UpdateChannelRequest{Archived: &archived})
}

func (archive *ChannelArchiveService) Restore(channelId string) {
	archive.choose(channelId)
	archived := false
	archive.Notice = archive.noticeOf(archive.update(UpdateChannelRequest{Archived: &archived}))
}

func (archive *ChannelArchiveService) choose(channelId string) Channel {
	channel := archive.channelList.Find(channelId)
	archive.ChannelId = channel.ChannelId
	archive.label = "#" + channel.Name
	archive.Notice = ""
	return channel
}

func (archive *ChannelArchiveService) update(request UpdateChannelRequest) string {
	archive.Busy = true
	answer, failure := archive.channelsClient.UpdateChannel(archive.session.Authorization(), archive.ChannelId, request)
	archive.Busy = false
	archive.closeWhenSaved(answer, failure)
	return archive.problemText(archive.problems.Of(failure))
}

func (archive *ChannelArchiveService) closeWhenSaved(answer Channel, failure error) {
	if failure != nil {
		return
	}
	archive.channelList.Replace(answer)
	archive.Open = false
}

func (archive *ChannelArchiveService) addressNoteOf(address string) string {
	if address == "" {
		return archive.label + " has no email address. When a channel with an address is archived, mail to that address is refused."
	}
	return "Mail to " + address + " is refused while " + archive.label + " is archived."
}

func (archive *ChannelArchiveService) noticeOf(problem string) string {
	if problem == "" {
		return ""
	}
	return archive.label + " is still archived. " + problem
}

func (archive *ChannelArchiveService) problemText(problem string) string {
	text, known := archive.problemTexts()[problem]
	if !known {
		return "Mesail could not reach the server. Check your connection, then try again."
	}
	return text
}

func (archive *ChannelArchiveService) problemTexts() map[string]string {
	return map[string]string{
		"":          "",
		"forbidden": "Only a domain admin can archive channels.",
		"notFound":  "This channel no longer exists.",
		"signedOut": "Your session ended. Sign in again, then try again.",
	}
}
