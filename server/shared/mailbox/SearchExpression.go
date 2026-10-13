// SearchExpression is one node of a parsed search: a field term, a NOT of
// one expression, or an AND or OR of two.
type SearchExpression struct {
	Type       string
	Field      string
	Value      string
	Left       *SearchExpression
	Right      *SearchExpression
	Expression *SearchExpression
}

// HasField reports whether the expression holds a term of the field.
func (expression *SearchExpression) HasField(field string) bool {
	switch {
	case expression == nil:
		return false
	case expression.Type == "term":
		return expression.Field == field
	case expression.Type == "not":
		return expression.Expression.HasField(field)
	}
	return expression.Left.HasField(field) || expression.Right.HasField(field)
}
