package main

import "strings"

const layerOrder = "@layer duvay, theme, base, components, utilities;"

func withLayerOrder(original string) string {
	first, rest, _ := strings.Cut(original, "\n")
	if isLayerStatement(first) {
		return layerOrder + "\n" + rest
	}
	return layerOrder + "\n\n" + original
}

func isLayerStatement(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "@layer ") && strings.HasSuffix(trimmed, ";") && !strings.Contains(trimmed, "{")
}
