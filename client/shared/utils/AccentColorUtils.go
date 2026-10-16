import "cmp"

// AccentColorUtils lists the accent presets that the application shell
// applies to the document.
type AccentColorUtils struct{}

func (AccentColorUtils) Presets() []AccentPreset {
	return []AccentPreset{
		{Key: "teal", Label: "Teal (default)", Primary: "#1f6f8b", Container: "#d7edf4", OnContainer: "#073544"},
		{Key: "blue", Label: "Blue", Primary: "#2562d4", Container: "#dbe7ff", OnContainer: "#0a2d75"},
		{Key: "purple", Label: "Purple", Primary: "#6b3aa0", Container: "#ebdcff", OnContainer: "#341760"},
		{Key: "pink", Label: "Pink", Primary: "#c2185b", Container: "#ffd9e6", OnContainer: "#5b0a28"},
		{Key: "orange", Label: "Orange", Primary: "#d97706", Container: "#ffe4c2", OnContainer: "#5a2e00"},
		{Key: "green", Label: "Green", Primary: "#2e7d32", Container: "#cfe9d0", OnContainer: "#0e3911"},
		{Key: "red", Label: "Red", Primary: "#c0392b", Container: "#ffd9d4", OnContainer: "#5a120a"},
		{Key: "slate", Label: "Slate", Primary: "#475569", Container: "#dbe3ee", OnContainer: "#1c2735"},
	}
}

// Label answers the preset label, or the key itself when it is unknown.
func (utils AccentColorUtils) Label(key string) string {
	return cmp.Or(utils.labels()[key], key)
}

func (utils AccentColorUtils) Known(key string) bool {
	return utils.labels()[key] != ""
}

func (utils AccentColorUtils) labels() map[string]string {
	labels := map[string]string{}
	for _, preset := range utils.Presets() {
		labels[preset.Key] = preset.Label
	}
	return labels
}
