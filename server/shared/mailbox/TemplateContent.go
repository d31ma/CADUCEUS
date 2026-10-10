import "regexp"

// TemplateContent validates the owner-editable fields of a message
// template: name, subject, sanitized text and the To and Cc lines.
type TemplateContent struct{}

// Normalized answers the fields of input (only those it names when partial)
// or why it is refused.
func (content TemplateContent) Normalized(input any, partial bool) (map[string]any, string) {
	source, isObject := RecordValues{}.Object(input)
	if !isObject {
		return nil, "Template payload is required"
	}
	patch := map[string]any{}
	for _, key := range []string{"name", "subject", "text", "to", "cc"} {
		value, present := source[key]
		if !present {
			continue
		}
		if _, isText := value.(string); !isText {
			return nil, key + " must be text"
		}
		patch[key] = value
	}
	if problem := content.normalize(patch, partial); problem != "" {
		return nil, problem
	}
	if partial && len(patch) == 0 {
		return nil, "At least one template field is required"
	}
	return patch, ""
}

func (TemplateContent) normalize(patch map[string]any, partial bool) string {
	field := func(key string) (string, bool) {
		value, present := patch[key]
		return TextValue{}.Of(value), present
	}
	if name, present := field("name"); !partial || present {
		normalized, problem := ContentNames{}.Normalized(name)
		if problem != "" {
			return problem
		}
		patch["name"] = normalized
	}
	control := regexp.MustCompile(`[\r\n\x00]`)
	if subject, present := field("subject"); !partial || present {
		subject = Security{}.Trim(subject)
		if (TextValue{}).Length(subject) > 998 || control.MatchString(subject) {
			return "subject is invalid"
		}
		patch["subject"] = subject
	}
	if raw, present := field("text"); !partial || present {
		text := StoredHtml{}.Sanitized(raw, false)
		if !partial && (Security{}).Trim(text) == "" {
			return "template content is required"
		}
		if len(text) > 512*1024 {
			return "template content exceeds 512 KiB"
		}
		patch["text"] = text
	}
	for _, key := range []string{"to", "cc"} {
		if value, present := field(key); !partial || present {
			value = Security{}.Trim(value)
			if (TextValue{}).Length(value) > 8_192 || control.MatchString(value) {
				return key + " is invalid"
			}
			patch[key] = value
		}
	}
	return ""
}
