import (
	"slices"
	"strings"
	"unicode"
)

// @Service
type SearchHighlightService struct{}

func (service *SearchHighlightService) PartsOf(snippet string, query string) []SearchHitPart {
	return service.marked(service.pieces(snippet), service.terms(query))
}

func (service *SearchHighlightService) terms(query string) []string {
	return strings.FieldsFunc(strings.ToLower(query), service.isSeparator)
}

func (service *SearchHighlightService) pieces(snippet string) []string {
	pieces := []string{""}
	for _, character := range snippet {
		pieces = service.extend(pieces, character)
	}
	return slices.DeleteFunc(pieces, service.isEmpty)
}

func (service *SearchHighlightService) extend(pieces []string, character rune) []string {
	if service.isSeparator(service.lastRune(pieces)) == service.isSeparator(character) {
		pieces[len(pieces)-1] += string(character)
		return pieces
	}
	return append(pieces, string(character))
}

func (service *SearchHighlightService) lastRune(pieces []string) rune {
	letters := []rune(service.startMark() + pieces[len(pieces)-1])
	return letters[len(letters)-1]
}

func (service *SearchHighlightService) marked(pieces []string, terms []string) []SearchHitPart {
	parts := []SearchHitPart{}
	for _, piece := range pieces {
		parts = append(parts, SearchHitPart{Text: piece, Match: service.matches(piece, terms)})
	}
	return parts
}

func (service *SearchHighlightService) matches(piece string, terms []string) bool {
	lowered := strings.ToLower(piece)
	return slices.ContainsFunc(terms, func(term string) bool {
		return strings.HasPrefix(lowered, term)
	})
}

func (service *SearchHighlightService) isEmpty(piece string) bool {
	return piece == ""
}

func (service *SearchHighlightService) isSeparator(character rune) bool {
	return !unicode.In(character, unicode.Letter, unicode.Digit)
}

func (service *SearchHighlightService) startMark() string {
	return "\x00"
}
