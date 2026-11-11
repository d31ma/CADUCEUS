package core

import "encoding/json"

type AttachmentCodec struct{}

func (codec AttachmentCodec) Encode(refs []AttachmentRef) string {
	encoded, _ := json.Marshal(append([]AttachmentRef{}, refs...))
	return string(encoded)
}

func (codec AttachmentCodec) Refs(message Message) []AttachmentRef {
	refs := []AttachmentRef{}
	failure := json.Unmarshal([]byte(message.AttachmentsJson), &refs)
	return codec.orIdsAlone(failure, refs, message.AttachmentIds)
}

func (codec AttachmentCodec) orIdsAlone(failure error, refs []AttachmentRef, attachmentIds []string) []AttachmentRef {
	if failure == nil {
		return refs
	}
	return codec.idsAlone(attachmentIds)
}

func (codec AttachmentCodec) idsAlone(attachmentIds []string) []AttachmentRef {
	refs := []AttachmentRef{}
	for _, attachmentId := range attachmentIds {
		refs = append(refs, AttachmentRef{AttachmentId: attachmentId})
	}
	return refs
}
