import (
	"fmt"
	"math"
	"regexp"
)

// AttachmentUtils formats attachment sizes for the attachment pills and
// names inline images, like createInlineContentId in rich-compose-editor.js.
type AttachmentUtils struct{}

// SizeLabel writes 512 B, 12 KB or 1.5 MB.
func (AttachmentUtils) SizeLabel(size int) string {
	return AttachmentUtils{}.label(float64(size), AttachmentUtils{}.kilobyte())
}

// InlineContentID is the RFC Content-ID of an inline image, or "" for a
// regular attachment.
func (AttachmentUtils) InlineContentID(inline bool, id string) string {
	return AttachmentUtils{}.contentID(inline, regexp.MustCompile(`[^a-zA-Z0-9.-]`).ReplaceAllString(id, ""))
}

func (AttachmentUtils) kilobyte() float64 { return 1024 }

func (AttachmentUtils) label(size float64, unit float64) string {
	if size < unit {
		return fmt.Sprintf("%d B", int(size))
	} else if size < unit*unit {
		return fmt.Sprintf("%d KB", int(math.Round(size/unit)))
	}
	return fmt.Sprintf("%.1f MB", size/unit/unit)
}

func (AttachmentUtils) contentID(inline bool, id string) string {
	if inline {
		return "caduceus." + id + "@inline"
	}
	return ""
}
