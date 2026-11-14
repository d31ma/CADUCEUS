package media

type ViewerState struct {
	DeviceId   string
	VideoShown []string
	AudioShown []string
	ScreenSlot bool
	Bitrate    int
}
