import "strings"

// SignatureContent validates the owner-editable fields of a signature: its
// name, its sanitized HTML or text, and the exact owned addresses it is the
// default for. Identity fields and timestamps are ignored; "domain" is
// accepted as a hint that selects the owned addresses of that domain.
type SignatureContent struct{}

// Normalized answers the fields of input (only those it names when partial)
// or why it is refused.
func (content SignatureContent) Normalized(input any, partial bool, allowedAddresses []string) (map[string]any, string) {
	source, isObject := RecordValues{}.Object(input)
	if !isObject {
		return nil, "Signature payload is required"
	}
	result := map[string]any{}
	_, hasName := source["name"]
	if !partial || hasName {
		text, isText := source["name"].(string)
		if !isText {
			return nil, "name must be text"
		}
		name, problem := ContentNames{}.Normalized(text)
		if problem != "" {
			return nil, problem
		}
		result["name"] = name
	}
	_, hasText := source["text"]
	if !partial || hasText {
		text, problem := content.text(source["text"])
		if problem != "" {
			return nil, problem
		}
		result["text"] = text
	}
	_, hasDefaults := source["defaultFor"]
	_, hasDomain := source["domain"]
	if !partial || hasDefaults || hasDomain {
		defaults, problem := content.defaults(content.requestedDefaults(source, allowedAddresses), allowedAddresses)
		if problem != "" {
			return nil, problem
		}
		result["defaultFor"] = defaults
	}
	if partial && len(result) == 0 {
		return nil, "At least one signature field is required"
	}
	return result, ""
}

func (SignatureContent) text(value any) (string, string) {
	raw, isText := value.(string)
	if !isText {
		return "", "text must be HTML or plain text"
	}
	text := StoredHtml{}.Sanitized(raw, true)
	if (Security{}).Trim(text) == "" {
		return "", "signature content is required"
	}
	if len(text) > 512*1024 {
		return "", "signature content exceeds 512 KiB"
	}
	return text, ""
}

func (SignatureContent) requestedDefaults(source map[string]any, allowedAddresses []string) any {
	domainText, isText := source["domain"].(string)
	if source["defaultFor"] != nil || !isText {
		return source["defaultFor"]
	}
	domain := Security{}.NormalizeDomain(domainText)
	if domain == "" {
		domain = "null"
	}
	selected := []any{}
	for _, address := range allowedAddresses {
		if strings.HasSuffix(address, "@"+domain) {
			selected = append(selected, address)
		}
	}
	return selected
}

func (SignatureContent) defaults(value any, allowedAddresses []string) ([]string, string) {
	requested, isList := RecordValues{}.List(RecordValues{}.Coalesce(value, []any{}))
	if !isList || len(requested) > 100 {
		return nil, "defaultFor must be an array of owned email addresses"
	}
	defaults := []string{}
	for _, address := range requested {
		normalized := Security{}.NormalizeEmailAddress(TextValue{}.Of(address))
		if normalized == "" || !(PreferenceChoices{}).Includes(allowedAddresses, normalized) {
			return nil, "defaultFor contains an address not owned by this mailbox"
		}
		if !(PreferenceChoices{}).Includes(defaults, normalized) {
			defaults = append(defaults, normalized)
		}
	}
	return defaults, ""
}
