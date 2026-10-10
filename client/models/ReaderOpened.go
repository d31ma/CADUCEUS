type ReaderOpened struct {
	Path               string `json:"path"`
	Search             string `json:"search"`
	Origin             string `json:"origin"`
	Theme              string `json:"theme"`
	TranslationTarget  string `json:"translationTarget"`
	ParticipantSidebar bool   `json:"participantSidebar"`
}
