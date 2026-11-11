import (
	"cmp"
	"math/bits"
	"strconv"
	"strings"
)

// @Service
type ConversationAttachmentService struct{}

func (service *ConversationAttachmentService) Cards(attachments []AttachmentRef) []AttachmentCard {
	return service.cardsOf(attachments)
}

func (service *ConversationAttachmentService) cardsOf(attachments []AttachmentRef) []AttachmentCard {
	cards := []AttachmentCard{}
	for _, attachment := range attachments {
		cards = append(cards, service.card(attachment))
	}
	return cards
}

func (service *ConversationAttachmentService) card(attachment AttachmentRef) AttachmentCard {
	return AttachmentCard{
		AttachmentId: attachment.AttachmentId,
		FileName:     cmp.Or(attachment.FileName, service.unnamedLabel()),
		TypeLabel:    service.typeLabel(attachment.MediaType),
		SizeLabel:    service.sizeLabel(attachment.Size),
		Details:      service.detailsOf(attachment),
	}
}

func (service *ConversationAttachmentService) detailsOf(attachment AttachmentRef) string {
	if attachment.FileName == "" {
		return ""
	}
	return service.typeLabel(attachment.MediaType) + " · " + service.sizeLabel(attachment.Size)
}

func (service *ConversationAttachmentService) unnamedLabel() string {
	return "Attachment"
}

func (service *ConversationAttachmentService) sizeLabel(size int64) string {
	positive := max(size, 0)
	unit := min((bits.Len64(uint64(positive))-1)/service.bitsPerUnit(), len(service.unitNames())-1)
	value := float64(positive) / float64(int64(1)<<(service.bitsPerUnit()*unit))
	return strconv.FormatFloat(value, 'f', service.decimals()[unit], 64) + " " + service.unitNames()[unit]
}

func (service *ConversationAttachmentService) typeLabel(mediaType string) string {
	clean := strings.ToLower(strings.TrimSpace(strings.Split(mediaType, ";")[0]))
	family, _, _ := strings.Cut(clean, "/")
	return cmp.Or(service.exactTypes()[clean], service.familyTypes()[family], "File")
}

func (service *ConversationAttachmentService) exactTypes() map[string]string {
	return map[string]string{
		"application/pdf":          "PDF",
		"text/csv":                 service.spreadsheetLabel(),
		"application/vnd.ms-excel": service.spreadsheetLabel(),
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": service.spreadsheetLabel(),
		"application/vnd.oasis.opendocument.spreadsheet":                    service.spreadsheetLabel(),
		"application/zip":              service.archiveLabel(),
		"application/x-zip-compressed": service.archiveLabel(),
		"application/gzip":             service.archiveLabel(),
		"application/x-tar":            service.archiveLabel(),
		"application/x-7z-compressed":  service.archiveLabel(),
		"application/vnd.rar":          service.archiveLabel(),
	}
}

func (service *ConversationAttachmentService) familyTypes() map[string]string {
	return map[string]string{"image": "Image", "text": "Text"}
}

func (service *ConversationAttachmentService) unitNames() []string {
	return []string{"B", "KB", "MB", "GB"}
}

func (service *ConversationAttachmentService) decimals() []int {
	return []int{0, 0, 1, 1}
}

func (service *ConversationAttachmentService) bitsPerUnit() int {
	return 10
}

func (service *ConversationAttachmentService) spreadsheetLabel() string {
	return "Spreadsheet"
}

func (service *ConversationAttachmentService) archiveLabel() string {
	return "Archive"
}
