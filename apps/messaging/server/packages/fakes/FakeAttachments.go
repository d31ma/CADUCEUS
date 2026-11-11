package fakes

import "mesail.dev/packages/core"

type FakeAttachments struct {
	Infos   map[string]core.AttachmentInfo
	Asked   []string
	Failure error
}

func NewFakeAttachments() *FakeAttachments {
	return &FakeAttachments{Infos: map[string]core.AttachmentInfo{}}
}

func (attachments *FakeAttachments) Ready(domainId string, attachmentId string, fileName string, size int64) {
	attachments.Infos[domainId+"/"+attachmentId] = core.AttachmentInfo{AttachmentId: attachmentId, FileName: fileName, MediaType: "application/octet-stream", Size: size, State: "ready"}
}

func (attachments *FakeAttachments) Describe(domainId string, attachmentId string) (core.AttachmentInfo, bool, error) {
	attachments.Asked = append(attachments.Asked, attachmentId)
	info, found := attachments.Infos[domainId+"/"+attachmentId]
	return info, found, attachments.Failure
}
