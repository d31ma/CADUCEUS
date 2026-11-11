package richtext

import "strings"

type listMarker struct {
	indent  int
	ordered bool
	content string
	found   bool
}

func (marker listMarker) parse(line string) listMarker {
	body := strings.TrimLeft(line, marker.blanks())
	bullet := marker.bulletLength(body)
	number := marker.numberLength(body)
	rest := body[bullet+number:]
	return listMarker{
		indent:  marker.indentOf(line[:len(line)-len(body)]),
		ordered: number > 0,
		content: strings.TrimLeft(rest, marker.blanks()),
		found:   bullet+number > 0 && marker.separated(rest),
	}
}

func (listMarker) blanks() string {
	return " \t"
}

func (listMarker) maxDigits() int {
	return 9
}

func (listMarker) indentOf(prefix string) int {
	return len(prefix) + 3*strings.Count(prefix, "\t")
}

func (listMarker) bulletLength(body string) int {
	if !strings.HasPrefix(body, "-") && !strings.HasPrefix(body, "*") && !strings.HasPrefix(body, "+") {
		return 0
	}
	return 1
}

func (marker listMarker) numberLength(body string) int {
	digits := len(body) - len(strings.TrimLeft(body, "0123456789"))
	after := body[digits:]
	if digits == 0 || digits > marker.maxDigits() || !strings.ContainsAny(after[:min(len(after), 1)], ".)") {
		return 0
	}
	return digits + 1
}

func (marker listMarker) separated(rest string) bool {
	return rest == "" || strings.TrimLeft(rest, marker.blanks()) != rest
}
