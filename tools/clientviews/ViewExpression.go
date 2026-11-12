package main

import (
	"regexp"
	"strings"
)

type ViewExpression struct {
	Text string
	Line int
}

func (expression ViewExpression) HasChoice() bool {
	return expression.choicePattern().MatchString(expression.withoutStrings())
}

func (expression ViewExpression) Problem(file string) Problem {
	return Problem{File: file, Line: expression.Line, Reason: expression.choiceReason()}
}

func (expression ViewExpression) choiceReason() string {
	return "the view holds a ? : expression (" + strings.TrimSpace(expression.Text) + "); give the result a service field and show that field"
}

func (ViewExpression) choicePattern() *regexp.Regexp {
	return regexp.MustCompile(`(?:^|[^?])\?(?:$|[^?.]|\.[0-9])`)
}

func (expression ViewExpression) withoutStrings() string {
	return expression.stringPattern().ReplaceAllString(expression.Text, "''")
}

func (ViewExpression) stringPattern() *regexp.Regexp {
	return regexp.MustCompile(`'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*"|` + "`(?:[^`\\\\]|\\\\.)*`")
}
