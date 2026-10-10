// TranslationPartUtils gathers a translated text that the translation engine
// sends in bounded parts, because one call into a service holds at most
// 64 KiB.
type TranslationPartUtils struct{}

func (TranslationPartUtils) With(parts []string, part TranslationPart) []string {
	sized := TranslationPartUtils{}.sized(parts, part.Count)
	sized[part.Index] = part.Text
	return sized
}

func (TranslationPartUtils) sized(parts []string, count int) []string {
	if len(parts) != count {
		return make([]string, count)
	}
	return parts
}
