package main

import (
	"slices"
	"strings"
)

type BlockedPaths struct {
	Prefixes []string
}

func (blocked BlockedPaths) Covers(path string) bool {
	return slices.ContainsFunc(blocked.Prefixes, func(prefix string) bool { return strings.HasPrefix(path, prefix) })
}
