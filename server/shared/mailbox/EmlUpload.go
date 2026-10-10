import (
	"errors"
	"regexp"
	"strings"
)

// EmlUpload validates the body of POST /inbox/import/eml: a destination
// folder other than drafts and one base64 EML file of at most 50 MiB that
// starts with a recognisable RFC 822 header block.
type EmlUpload struct{}

func (EmlUpload) largestFile() int { return 50 * 1024 * 1024 }

// Folder answers the destination folder, or why it is invalid.
func (EmlUpload) Folder(value any) (string, error) {
	text, isText := value.(string)
	if !isText || (TextValue{}).Trim(text) == "" {
		return "", errors.New("folder is required")
	}
	folder := strings.ToLower(TextValue{}.Trim(text))
	if folder == "drafts" {
		return "", errors.New("EML files cannot be imported as drafts")
	}
	if !regexp.MustCompile(`^[a-z0-9_-]{1,60}$`).MatchString(folder) {
		return "", errors.New("folder is invalid")
	}
	return folder, nil
}

// Source answers the decoded file, or why it is invalid.
func (upload EmlUpload) Source(value any) ([]byte, error) {
	text, isText := value.(string)
	if !isText || text == "" || !regexp.MustCompile(`^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`).MatchString(text) {
		return nil, errors.New("contentBase64 must contain one EML file")
	}
	if len(text) > (upload.largestFile()+2)/3*4 {
		return nil, errors.New("EML file exceeds 50 MiB")
	}
	source := Base64Text{}.Decoded(text)
	if len(source) == 0 || len(source) > upload.largestFile() {
		return nil, errors.New("EML file exceeds 50 MiB")
	}
	if !upload.looksLikeEml(source) {
		return nil, errors.New("File is not a recognizable RFC 822 message")
	}
	return source, nil
}

// looksLikeEml reports whether the first 256 KiB hold a header block with a
// common message field.
func (EmlUpload) looksLikeEml(source []byte) bool {
	header := MimeCharset{}.Text("latin1", source[:min(len(source), 256*1024)])
	separator := regexp.MustCompile(`\r?\n\r?\n`).FindStringIndex(header)
	if separator == nil || separator[0] <= 0 {
		return false
	}
	block := regexp.MustCompile(`\r?\n[ \t]+`).ReplaceAllString(header[:separator[0]], " ")
	return regexp.MustCompile(`(?im)^(?:from|to|cc|date|subject|message-id|mime-version|content-type):`).MatchString(block)
}
