import (
	"html"
	"strings"
)

// HtmlAttributes reads the start tags of an HTML document the way an HTML
// parser sees them: lower-case tag and attribute names, entity-decoded
// attribute values, comments and end tags skipped, and the text of style
// elements kept apart.
type HtmlAttributes struct {
	Tags   []map[string]string
	Styles []string
}

// Parsed answers the start tags and style texts of document.
func (HtmlAttributes) Parsed(document string) HtmlAttributes {
	parsed := HtmlAttributes{}
	lower := strings.ToLower(document)
	for index := 0; index < len(document); {
		start := strings.IndexByte(document[index:], '<')
		if start < 0 {
			break
		}
		index += start
		switch {
		case strings.HasPrefix(document[index:], "<!--"):
			end := strings.Index(document[index+4:], "-->")
			if end < 0 {
				return parsed
			}
			index += 4 + end + 3
		case strings.HasPrefix(document[index:], "</") || strings.HasPrefix(document[index:], "<!") || strings.HasPrefix(document[index:], "<?"):
			end := strings.IndexByte(document[index:], '>')
			if end < 0 {
				return parsed
			}
			index += end + 1
		default:
			name, attributes, next := parsed.tag(document, index+1)
			index = next
			if name == "" {
				continue
			}
			parsed.Tags = append(parsed.Tags, attributes)
			if name == "style" || name == "script" || name == "textarea" || name == "title" {
				end := strings.Index(lower[index:], "</"+name)
				if end < 0 {
					end = len(document) - index
				}
				if name == "style" {
					parsed.Styles = append(parsed.Styles, document[index:index+end])
				}
				index += end
			}
		}
	}
	return parsed
}

// tag reads one start tag from just after its "<" and answers its name, its
// attributes and the offset after it ("" for text that is no tag).
func (HtmlAttributes) tag(document string, index int) (string, map[string]string, int) {
	start := index
	for index < len(document) && !strings.ContainsRune(" \t\n\r\f/>", rune(document[index])) {
		index++
	}
	name := strings.ToLower(document[start:index])
	if name == "" || !(name[0] >= 'a' && name[0] <= 'z') {
		return "", nil, start
	}
	attributes := map[string]string{}
	for index < len(document) {
		for index < len(document) && strings.ContainsRune(" \t\n\r\f/", rune(document[index])) {
			index++
		}
		if index >= len(document) || document[index] == '>' {
			return name, attributes, index + 1
		}
		keyStart := index
		for index < len(document) && !strings.ContainsRune(" \t\n\r\f/>=", rune(document[index])) {
			index++
		}
		key := strings.ToLower(document[keyStart:index])
		for index < len(document) && strings.ContainsRune(" \t\n\r\f", rune(document[index])) {
			index++
		}
		value := ""
		if index < len(document) && document[index] == '=' {
			value, index = HtmlAttributes{}.value(document, index+1)
		}
		if _, seen := attributes[key]; !seen && key != "" {
			attributes[key] = html.UnescapeString(value)
		}
	}
	return name, attributes, index
}

func (HtmlAttributes) value(document string, index int) (string, int) {
	for index < len(document) && strings.ContainsRune(" \t\n\r\f", rune(document[index])) {
		index++
	}
	if index < len(document) && (document[index] == '"' || document[index] == '\'') {
		quote := document[index]
		end := strings.IndexByte(document[index+1:], quote)
		if end < 0 {
			return document[index+1:], len(document)
		}
		return document[index+1 : index+1+end], index + end + 2
	}
	start := index
	for index < len(document) && !strings.ContainsRune(" \t\n\r\f>", rune(document[index])) {
		index++
	}
	return document[start:index], index
}
