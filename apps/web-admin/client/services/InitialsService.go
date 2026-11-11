import "strings"

// @Service
type InitialsService struct{}

func (initials *InitialsService) Of(name string) string {
	words := append(strings.Fields(name), "", "")
	return strings.ToUpper(initials.firstLetter(words[0]) + initials.firstLetter(words[1]))
}

func (initials *InitialsService) firstLetter(word string) string {
	letters := []rune(word)
	return string(letters[:min(1, len(letters))])
}
