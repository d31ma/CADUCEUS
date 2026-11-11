package realtime

type SettingsReader struct {
	Environment func(string) string
}

func (reader SettingsReader) Read() Settings {
	return Settings{}
}
