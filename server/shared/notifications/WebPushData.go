// WebPushData is the navigation data of a new-mail notification. The message
// id is opaque app navigation data and is reauthorized before any action.
type WebPushData struct {
	Url     string `json:"url"`
	EmailId string `json:"emailId"`
	SoundId string `json:"soundId"`
}
