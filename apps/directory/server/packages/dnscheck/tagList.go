package dnscheck

import "strings"

type tagList struct{}

func (list tagList) values(record string) map[string]string {
	values := map[string]string{}
	for _, piece := range strings.Split(record, list.separator()) {
		name, value := list.pair(piece)
		values[name] = value
	}
	return values
}

func (list tagList) first(record string) string {
	piece, _, _ := strings.Cut(record, list.separator())
	name, value := list.pair(piece)
	return name + list.equals() + value
}

func (list tagList) pair(piece string) (string, string) {
	name, value, _ := strings.Cut(piece, list.equals())
	return strings.TrimSpace(name), strings.TrimSpace(value)
}

func (tagList) separator() string {
	return ";"
}

func (tagList) equals() string {
	return "="
}
