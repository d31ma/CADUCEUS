import (
	"encoding/base64"
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// AttachmentContentUtils decides which attachment bytes may become inert
// previews and inline images, as attachment-content.js did.
type AttachmentContentUtils struct{}

func (AttachmentContentUtils) NormalizeContentReference(value string) string {
	normalized := AttachmentContentUtils{}.decoded(AttachmentContentUtils{}.withoutScheme(strings.TrimSpace(value)))
	return AttachmentContentUtils{}.boundedReference(AttachmentContentUtils{}.unbracketed(normalized))
}

func (AttachmentContentUtils) scheme() string { return "cid:" }

func (AttachmentContentUtils) IsContentReference(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), AttachmentContentUtils{}.scheme())
}

func (AttachmentContentUtils) IsRasterImage(contentType string) bool {
	return AttachmentContentUtils{}.rasterTypes()[AttachmentContentUtils{}.normalizedType(contentType)]
}

func (AttachmentContentUtils) PreviewKind(contentType string) string {
	return AttachmentContentUtils{}.previewKind(AttachmentContentUtils{}.normalizedType(contentType))
}

func (AttachmentContentUtils) PreviewDataUrl(contentType string, contentBase64 string) string {
	normalized := AttachmentContentUtils{}.normalizedType(contentType)
	return AttachmentContentUtils{}.dataUrl(AttachmentContentUtils{}.previewKind(normalized), normalized, contentBase64)
}

func (AttachmentContentUtils) DecodeText(contentBase64 string) (string, error) {
	bytes, failure := AttachmentContentUtils{}.decodedBytes(contentBase64)
	return AttachmentContentUtils{}.boundedText(bytes, failure)
}

func (AttachmentContentUtils) PreviewFailure(kind string, previewUrl string, failure error, textFailure error) string {
	return AttachmentContentUtils{}.previewFailure(kind, previewUrl, []error{failure, textFailure})
}

func (AttachmentContentUtils) WithInlineSource(sources map[string]string, contentId string, source string) map[string]string {
	reference := AttachmentContentUtils{}.NormalizeContentReference(contentId)
	_, duplicate := sources[reference]
	return AttachmentContentUtils{}.withSource(sources, reference, AttachmentContentUtils{}.uniqueSource(duplicate, source))
}

func (AttachmentContentUtils) rasterTypes() map[string]bool {
	return map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true, "image/avif": true}
}

func (AttachmentContentUtils) textTypes() map[string]bool {
	return map[string]bool{"application/json": true, "application/xml": true, "application/yaml": true, "application/x-yaml": true}
}

func (AttachmentContentUtils) base64Pattern() *regexp.Regexp {
	return regexp.MustCompile(`^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$`)
}

func (AttachmentContentUtils) normalizedType(value string) string {
	if strings.TrimSpace(value) == "" {
		return "application/octet-stream"
	}
	return strings.ToLower(strings.TrimSpace(strings.SplitN(value, ";", 2)[0]))
}

func (AttachmentContentUtils) withoutScheme(value string) string {
	if strings.HasPrefix(strings.ToLower(value), AttachmentContentUtils{}.scheme()) {
		return value[len(AttachmentContentUtils{}.scheme()):]
	}
	return value
}

func (AttachmentContentUtils) decoded(value string) string {
	text, failure := url.PathUnescape(value)
	if failure != nil {
		return value
	}
	return text
}

func (AttachmentContentUtils) unbracketed(value string) string {
	if len(value) > 1 && strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">") {
		return strings.TrimSpace(value[1 : len(value)-1])
	}
	return value
}

func (AttachmentContentUtils) maximumReferenceLength() int { return 255 }

func (AttachmentContentUtils) boundedReference(value string) string {
	utils := AttachmentContentUtils{}
	unsafe := regexp.MustCompile(`[\s<>\x{0000}-\x{001f}\x{007f}-\x{009f}]`)
	if value == "" || utf8.RuneCountInString(value) > utils.maximumReferenceLength() || unsafe.MatchString(value) {
		return ""
	}
	return strings.ToLower(value)
}

func (AttachmentContentUtils) imageKind() string { return "image" }

func (AttachmentContentUtils) pdfKind() string { return "pdf" }

func (AttachmentContentUtils) textKind() string { return "text" }

func (AttachmentContentUtils) previewKind(normalized string) string {
	switch {
	case AttachmentContentUtils{}.rasterTypes()[normalized]:
		return AttachmentContentUtils{}.imageKind()
	case normalized == "application/pdf":
		return AttachmentContentUtils{}.pdfKind()
	case AttachmentContentUtils{}.isTextual(normalized):
		return AttachmentContentUtils{}.textKind()
	}
	return ""
}

func (AttachmentContentUtils) isTextual(normalized string) bool {
	return strings.HasPrefix(normalized, "text/") || AttachmentContentUtils{}.textTypes()[normalized]
}

func (AttachmentContentUtils) dataUrl(kind string, normalized string, contentBase64 string) string {
	utils := AttachmentContentUtils{}
	previewable := slices.Contains([]string{utils.imageKind(), utils.pdfKind()}, kind)
	valid := utils.base64Pattern().MatchString(contentBase64)
	if !previewable || contentBase64 == "" || !valid {
		return ""
	}
	return "data:" + normalized + ";base64," + contentBase64
}

func (AttachmentContentUtils) decodedBytes(contentBase64 string) ([]byte, error) {
	utils := AttachmentContentUtils{}
	if contentBase64 == "" || !utils.base64Pattern().MatchString(contentBase64) {
		return nil, errors.New("Attachment content is not valid base64.")
	}
	return base64.StdEncoding.DecodeString(contentBase64)
}

func (AttachmentContentUtils) maximumTextBytes() int { return 1048576 }

func (AttachmentContentUtils) boundedText(bytes []byte, failure error) (string, error) {
	switch {
	case failure != nil:
		return "", failure
	case len(bytes) > AttachmentContentUtils{}.maximumTextBytes():
		return "", errors.New("This text attachment is too large to preview.")
	}
	return strings.ToValidUTF8(string(bytes), "�"), nil
}

func (AttachmentContentUtils) previewFailure(kind string, previewUrl string, failures []error) string {
	switch {
	case failures[0] != nil:
		return failures[0].Error()
	case kind == AttachmentContentUtils{}.textKind():
		return AttachmentContentUtils{}.message(failures[1])
	case previewUrl == "":
		return "This attachment cannot be previewed safely."
	}
	return ""
}

func (AttachmentContentUtils) message(failure error) string {
	if failure == nil {
		return ""
	}
	return failure.Error()
}

func (AttachmentContentUtils) uniqueSource(duplicate bool, source string) string {
	if duplicate {
		return ""
	}
	return source
}

func (AttachmentContentUtils) withSource(sources map[string]string, reference string, source string) map[string]string {
	if reference != "" {
		sources[reference] = source
	}
	return sources
}
