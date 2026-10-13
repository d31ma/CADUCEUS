import "errors"

// SearchLexer splits a search into words, quoted phrases, colons and
// parentheses. An unterminated quote is a syntax error.
type SearchLexer struct {
	source []rune
	offset int
}

// Scanned answers the tokens of source, ending with an end token.
func (SearchLexer) Scanned(source string) ([]SearchToken, error) {
	lexer := &SearchLexer{source: []rune(source)}
	tokens := []SearchToken{}
	for lexer.offset < len(lexer.source) {
		token, found, err := lexer.next()
		if err != nil {
			return nil, err
		}
		if found {
			tokens = append(tokens, token)
		}
	}
	return append(tokens, SearchToken{Type: "eof"}), nil
}

func (lexer *SearchLexer) next() (SearchToken, bool, error) {
	for lexer.offset < len(lexer.source) && (TextValue{}).Space(lexer.source[lexer.offset]) {
		lexer.offset++
	}
	if lexer.offset >= len(lexer.source) {
		return SearchToken{}, false, nil
	}
	single := map[rune]string{'(': "leftParen", ')': "rightParen", ':': "colon"}
	character := lexer.source[lexer.offset]
	if kind, isSingle := single[character]; isSingle {
		lexer.offset++
		return SearchToken{Type: kind, Value: string(character)}, true, nil
	}
	if character == '"' {
		return lexer.quoted()
	}
	start := lexer.offset
	for lexer.offset < len(lexer.source) && !lexer.stops(lexer.source[lexer.offset]) {
		lexer.offset++
	}
	return SearchToken{Type: "text", Value: string(lexer.source[start:lexer.offset])}, true, nil
}

func (SearchLexer) stops(character rune) bool {
	return (TextValue{}).Space(character) || character == '(' || character == ')' || character == ':'
}

func (lexer *SearchLexer) quoted() (SearchToken, bool, error) {
	lexer.offset++
	start := lexer.offset
	for lexer.offset < len(lexer.source) && lexer.source[lexer.offset] != '"' {
		lexer.offset++
	}
	if lexer.offset >= len(lexer.source) {
		return SearchToken{}, false, errors.New("Expected closing quote for quoted search term")
	}
	value := string(lexer.source[start:lexer.offset])
	lexer.offset++
	return SearchToken{Type: "text", Value: value}, true, nil
}
