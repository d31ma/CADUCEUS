import (
	"errors"
	"strings"
)

// SearchParser reads the structured search grammar: field:value terms,
// quoted phrases, OR, NOT, AND and parentheses, with AND binding loosest.
type SearchParser struct {
	tokens []SearchToken
	offset int
}

// Parsed answers the expression of raw, nil for an empty search, or a
// syntax error.
func (SearchParser) Parsed(raw string) (*SearchExpression, error) {
	if (TextValue{}).Trim(raw) == "" {
		return nil, nil
	}
	tokens, err := SearchLexer{}.Scanned(raw)
	if err != nil {
		return nil, err
	}
	parser := &SearchParser{tokens: tokens}
	if parser.peek().Type == "eof" {
		return nil, nil
	}
	expression, err := parser.and()
	if err == nil && parser.peek().Type != "eof" {
		err = errors.New("Unexpected search token " + parser.peek().Value)
	}
	return expression, err
}

// WithFallback answers the expression of raw, or one full-text term of the
// whole search when it does not parse.
func (parser SearchParser) WithFallback(raw string) *SearchExpression {
	expression, err := parser.Parsed(raw)
	if err == nil {
		return expression
	}
	if trimmed := (TextValue{}).Trim(raw); trimmed != "" {
		return &SearchExpression{Type: "term", Field: "text", Value: trimmed}
	}
	return nil
}

func (parser *SearchParser) and() (*SearchExpression, error) {
	left, err := parser.or()
	if err != nil {
		return nil, err
	}
	if kind := parser.peek().Type; kind == "eof" || kind == "rightParen" {
		return left, nil
	}
	if parser.wordIs("AND") {
		parser.take()
	}
	if parser.wordIs("NOT") {
		parser.take()
		negated, err := parser.and()
		if err != nil {
			return nil, err
		}
		return &SearchExpression{Type: "and", Left: left, Right: &SearchExpression{Type: "not", Expression: negated}}, nil
	}
	right, err := parser.and()
	if err != nil {
		return nil, err
	}
	return &SearchExpression{Type: "and", Left: left, Right: right}, nil
}

func (parser *SearchParser) or() (*SearchExpression, error) {
	left, err := parser.primary()
	if err != nil || !parser.wordIs("OR") {
		return left, err
	}
	parser.take()
	right, err := parser.or()
	if err != nil {
		return nil, err
	}
	return &SearchExpression{Type: "or", Left: left, Right: right}, nil
}

func (parser *SearchParser) primary() (*SearchExpression, error) {
	if parser.wordIs("NOT") {
		parser.take()
		negated, err := parser.primary()
		if err != nil {
			return nil, err
		}
		return &SearchExpression{Type: "not", Expression: negated}, nil
	}
	if parser.peek().Type != "leftParen" {
		return parser.term()
	}
	parser.take()
	expression, err := parser.and()
	if err != nil {
		return nil, err
	}
	if parser.peek().Type != "rightParen" {
		return nil, errors.New("Expected ) to close search group")
	}
	parser.take()
	return expression, nil
}

func (parser *SearchParser) term() (*SearchExpression, error) {
	first, err := parser.text("Expected a search term")
	if err != nil {
		return nil, err
	}
	if parser.peek().Type != "colon" {
		return &SearchExpression{Type: "term", Field: "text", Value: first}, nil
	}
	parser.take()
	value, err := parser.text("Expected a value after " + first + ":")
	if err != nil {
		return nil, err
	}
	return &SearchExpression{Type: "term", Field: strings.ToLower(first), Value: value}, nil
}

func (parser *SearchParser) text(message string) (string, error) {
	if parser.peek().Type != "text" {
		return "", errors.New(message)
	}
	return parser.take().Value, nil
}

func (parser *SearchParser) wordIs(value string) bool {
	return parser.peek().Type == "text" && strings.ToUpper(parser.peek().Value) == value
}

func (parser *SearchParser) peek() SearchToken {
	if parser.offset >= len(parser.tokens) {
		return SearchToken{Type: "eof"}
	}
	return parser.tokens[parser.offset]
}

func (parser *SearchParser) take() SearchToken {
	token := parser.peek()
	parser.offset++
	return token
}
