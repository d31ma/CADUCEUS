package textindex

import (
	"slices"
	"strings"
)

type Matcher struct{}

func (matcher Matcher) Matches(documentTerms []string, queryTerms []string) bool {
	return matcher.firstMissing(documentTerms, queryTerms) < 0
}

func (matcher Matcher) firstMissing(documentTerms []string, queryTerms []string) int {
	return slices.IndexFunc(queryTerms, func(term string) bool { return !matcher.found(documentTerms, term) })
}

func (Matcher) found(documentTerms []string, term string) bool {
	return slices.ContainsFunc(documentTerms, func(documentTerm string) bool { return strings.HasPrefix(documentTerm, term) })
}
