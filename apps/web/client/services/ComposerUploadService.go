import (
	"slices"
	"strconv"
)

// @Service
type ComposerUploadService struct {
	Items []UploadItem
	// @Needs
	attachments *AttachmentsClient
	// @Needs
	auth *ComposerAuthService
}

func (uploads *ComposerUploadService) Pick(picked UploadPicked) {
	uploads.Items = uploads.withoutFile(picked.FileName)
	item := uploads.itemOf(picked)
	item.Problem = uploads.limitProblem(picked.Size, uploads.usedBytes())
	uploads.Items = append(uploads.Items, uploads.createdUnlessRefused(item))
}

func (uploads *ComposerUploadService) Complete(attachmentId string) {
	authorization, failure := uploads.auth.Authorization()
	_, failure = uploads.completeUnlessFailed(authorization, attachmentId, failure)
	uploads.Items = uploads.withState(attachmentId, uploads.stateAfter(failure))
}

func (uploads *ComposerUploadService) Retry(fileName string) {
	item := uploads.find(fileName)
	uploads.Pick(UploadPicked{FileName: item.FileName, MediaType: item.MediaType, Size: item.Size, Sha256: item.Sha256})
}

func (uploads *ComposerUploadService) Remove(fileName string) {
	uploads.Items = uploads.withoutFile(fileName)
}

func (uploads *ComposerUploadService) Clear() {
	uploads.Items = []UploadItem{}
}

func (uploads *ComposerUploadService) AttachmentIds() []string {
	return uploads.idsOf(uploads.Items)
}

func (uploads *ComposerUploadService) CountInState(state string) int {
	return uploads.countState(state)
}

func (uploads *ComposerUploadService) UsedText() string {
	return uploads.megabytes(uploads.usedBytes()) + " of " + uploads.megabytes(uploads.messageLimit()) + " used"
}

func (uploads *ComposerUploadService) itemOf(picked UploadPicked) UploadItem {
	return UploadItem{FileName: picked.FileName, MediaType: picked.MediaType, Size: picked.Size, Sha256: picked.Sha256, SizeText: uploads.megabytes(picked.Size), State: uploads.stateUploading()}
}

func (uploads *ComposerUploadService) limitProblem(size int64, used int64) string {
	switch {
	case size > uploads.fileLimit():
		return "fileTooLarge"
	case used+size > uploads.messageLimit():
		return "messageTooLarge"
	}
	return ""
}

func (uploads *ComposerUploadService) createdUnlessRefused(item UploadItem) UploadItem {
	if item.Problem != "" {
		item.State = uploads.stateRefused()
		return item
	}
	return uploads.created(item)
}

func (uploads *ComposerUploadService) created(item UploadItem) UploadItem {
	authorization, failure := uploads.auth.Authorization()
	request := UploadCreateRequest{FileName: item.FileName, MediaType: item.MediaType, Size: item.Size, Sha256: item.Sha256}
	upload, failure := uploads.createUnlessFailed(authorization, request, failure)
	item.AttachmentId = upload.AttachmentId
	item.State = uploads.stateWhenCreated(failure)
	return item
}

func (uploads *ComposerUploadService) createUnlessFailed(authorization string, request UploadCreateRequest, failure error) (Upload, error) {
	if failure != nil {
		return Upload{}, failure
	}
	return BehaviorsAnswer[Upload](uploads.attachments.Create(authorization, request))
}

func (uploads *ComposerUploadService) completeUnlessFailed(authorization string, attachmentId string, failure error) (UploadComplete, error) {
	if failure != nil {
		return UploadComplete{}, failure
	}
	return BehaviorsAnswer[UploadComplete](uploads.attachments.Complete(authorization, attachmentId))
}

func (uploads *ComposerUploadService) stateWhenCreated(failure error) string {
	if failure != nil {
		return uploads.stateFailed()
	}
	return uploads.stateUploading()
}

func (uploads *ComposerUploadService) stateAfter(failure error) string {
	if failure != nil {
		return uploads.stateFailed()
	}
	return uploads.stateReady()
}

func (uploads *ComposerUploadService) withState(attachmentId string, state string) []UploadItem {
	items := []UploadItem{}
	for _, item := range uploads.Items {
		items = append(items, uploads.stateWhenMatching(item, attachmentId, state))
	}
	return items
}

func (uploads *ComposerUploadService) stateWhenMatching(item UploadItem, attachmentId string, state string) UploadItem {
	if item.AttachmentId != attachmentId {
		return item
	}
	item.State = state
	return item
}

func (uploads *ComposerUploadService) withoutFile(fileName string) []UploadItem {
	return slices.DeleteFunc(slices.Clone(uploads.Items), func(item UploadItem) bool {
		return item.FileName == fileName
	})
}

func (uploads *ComposerUploadService) find(fileName string) UploadItem {
	index := slices.IndexFunc(uploads.Items, func(item UploadItem) bool {
		return item.FileName == fileName
	})
	return uploads.itemAt(index)
}

func (uploads *ComposerUploadService) itemAt(index int) UploadItem {
	if index < 0 {
		return UploadItem{}
	}
	return uploads.Items[index]
}

func (uploads *ComposerUploadService) idsOf(items []UploadItem) []string {
	ids := []string{}
	for _, item := range items {
		ids = uploads.appendReady(ids, item)
	}
	return ids
}

func (uploads *ComposerUploadService) appendReady(ids []string, item UploadItem) []string {
	if item.State != uploads.stateReady() {
		return ids
	}
	return append(ids, item.AttachmentId)
}

func (uploads *ComposerUploadService) countState(state string) int {
	count := 0
	for _, item := range uploads.Items {
		count += uploads.oneWhen(item.State == state)
	}
	return count
}

func (uploads *ComposerUploadService) oneWhen(wanted bool) int {
	if wanted {
		return 1
	}
	return 0
}

func (uploads *ComposerUploadService) usedBytes() int64 {
	used := int64(0)
	for _, item := range uploads.Items {
		used += item.Size * int64(uploads.oneWhen(item.State != uploads.stateRefused()))
	}
	return used
}

func (uploads *ComposerUploadService) megabytes(size int64) string {
	return strconv.FormatFloat(float64(size)/float64(uploads.bytesPerMegabyte()), 'f', 1, 64) + " MB"
}

func (uploads *ComposerUploadService) fileLimit() int64 {
	return 25 * uploads.bytesPerMegabyte()
}

func (uploads *ComposerUploadService) messageLimit() int64 {
	return 50 * uploads.bytesPerMegabyte()
}

func (uploads *ComposerUploadService) bytesPerMegabyte() int64 {
	return 1048576
}

func (uploads *ComposerUploadService) stateUploading() string {
	return "uploading"
}

func (uploads *ComposerUploadService) stateFailed() string {
	return "failed"
}

func (uploads *ComposerUploadService) stateReady() string {
	return "ready"
}

func (uploads *ComposerUploadService) stateRefused() string {
	return "refused"
}
