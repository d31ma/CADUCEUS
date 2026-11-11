package sending

import (
	"cmp"

	"mesail.dev/packages/core"
)

type AttachmentCheck struct {
	catalog core.AttachmentCatalog
}

func NewAttachmentCheck(catalog core.AttachmentCatalog) AttachmentCheck {
	return AttachmentCheck{catalog: catalog}
}

func (check AttachmentCheck) refs(domainId string, attachmentIds []string) ([]core.AttachmentRef, error) {
	infos, err := check.described(domainId, attachmentIds)
	if err != nil {
		return nil, err
	}
	return check.converted(infos), check.sizeRefusal(check.total(infos))
}

func (check AttachmentCheck) described(domainId string, attachmentIds []string) ([]core.AttachmentInfo, error) {
	infos := []core.AttachmentInfo{}
	for _, attachmentId := range attachmentIds {
		info, err := check.usable(domainId, attachmentId)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (check AttachmentCheck) usable(domainId string, attachmentId string) (core.AttachmentInfo, error) {
	info, found, err := check.catalog.Describe(domainId, attachmentId)
	if err != nil {
		return info, Failures{}.attachments(err)
	}
	return info, cmp.Or(check.knownRefusal(found, attachmentId), check.readyRefusal(info))
}

func (check AttachmentCheck) knownRefusal(found bool, attachmentId string) error {
	if found {
		return nil
	}
	return Failures{}.invalid("no such attachment: "+attachmentId, check.field())
}

func (check AttachmentCheck) readyRefusal(info core.AttachmentInfo) error {
	if info.State == check.readyState() {
		return nil
	}
	return Failures{}.invalid("the attachment is not ready: "+info.AttachmentId, check.field())
}

func (AttachmentCheck) total(infos []core.AttachmentInfo) int64 {
	total := int64(0)
	for _, info := range infos {
		total += info.Size
	}
	return total
}

func (check AttachmentCheck) sizeRefusal(total int64) error {
	if total <= check.messageLimit() {
		return nil
	}
	return &core.TooLarge{Message: "the attachments of a message may have at most 50 MiB together"}
}

func (AttachmentCheck) converted(infos []core.AttachmentInfo) []core.AttachmentRef {
	refs := []core.AttachmentRef{}
	for _, info := range infos {
		refs = append(refs, core.AttachmentRef{AttachmentId: info.AttachmentId, FileName: info.FileName, MediaType: info.MediaType, Size: info.Size})
	}
	return refs
}

func (AttachmentCheck) messageLimit() int64 {
	return 52428800
}

func (AttachmentCheck) readyState() string {
	return "ready"
}

func (AttachmentCheck) field() string {
	return "attachmentIds"
}
