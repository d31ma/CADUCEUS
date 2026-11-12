package main

import (
	"regexp"
	"strings"
)

type ViewFile struct {
	Name   string
	Source string
}

func (file ViewFile) Problems() []Problem {
	return file.choiceProblems(file.expressions())
}

func (file ViewFile) expressions() []ViewExpression {
	expressions := []ViewExpression{}
	for _, match := range file.expressionPattern().FindAllStringSubmatchIndex(file.Source, -1) {
		expressions = append(expressions, file.expressionOf(match))
	}
	return expressions
}

func (file ViewFile) expressionOf(match []int) ViewExpression {
	group := file.textGroup(match)
	return file.expressionAt(match[group], match[group+1])
}

func (ViewFile) expressionPattern() *regexp.Regexp {
	return regexp.MustCompile(`\s(?:on)?:[A-Za-z][A-Za-z0-9_.:-]*\s*=\s*(?:"([^"]*)"|'([^']*)')|\{([^{}]*)\}`)
}

func (ViewFile) textGroup(match []int) int {
	for group := 2; group < len(match); group += 2 {
		if match[group] >= 0 {
			return group
		}
	}
	return 0
}

func (file ViewFile) expressionAt(start int, end int) ViewExpression {
	return ViewExpression{Text: file.Source[start:end], Line: strings.Count(file.Source[:start], "\n") + 1}
}

func (file ViewFile) choiceProblems(expressions []ViewExpression) []Problem {
	problems := []Problem{}
	for _, expression := range expressions {
		problems = append(problems, file.choiceProblem(expression)...)
	}
	return problems
}

func (file ViewFile) choiceProblem(expression ViewExpression) []Problem {
	if !expression.HasChoice() {
		return nil
	}
	return []Problem{expression.Problem(file.Name)}
}
