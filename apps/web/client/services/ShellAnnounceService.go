// @Service
type ShellAnnounceService struct {
	Message string
}

func (announcer *ShellAnnounceService) Announce(text string) {
	announcer.Message = text
}
