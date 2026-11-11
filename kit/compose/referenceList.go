package compose

import "strings"

type referenceList struct{}

func (referenceList) bare(id string) string {
	return strings.Trim(id, "<> ")
}

func (list referenceList) unique(ids []string) []string {
	seen := map[string]bool{"": true}
	result := []string{}
	for _, id := range ids {
		bareId := list.bare(id)
		if seen[bareId] {
			continue
		}
		seen[bareId] = true
		result = append(result, bareId)
	}
	return result
}

func (referenceList) capped(ids []string) []string {
	keep := limits{}.MaxReferences()
	if len(ids) <= keep {
		return ids
	}
	return append([]string{ids[0]}, ids[len(ids)-keep+1:]...)
}

func (list referenceList) bracketed(ids []string) string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		result = append(result, "<"+list.bare(id)+">")
	}
	return strings.Join(result, " ")
}
