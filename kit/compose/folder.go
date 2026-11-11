package compose

import "strings"

type folder struct{}

func (folder) fold(line string) string {
	width := limits{}.FoldWidth()
	words := strings.Split(line, " ")
	lines := []string{}
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+len(" ")+len(word) > width {
			lines = append(lines, current)
			current = word
			continue
		}
		current = current + " " + word
	}
	lines = append(lines, current)
	return strings.Join(lines, limits{}.LineEnd()+" ")
}
