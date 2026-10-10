import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AttachmentsRecords stores attachment files and their records and reads
// them back, including the legacy records that point at a file path.
type AttachmentsRecords struct{}

// ToSummary answers the fields of a record that routes show.
func (AttachmentsRecords) ToSummary(attachment map[string]any) map[string]any {
	summary := map[string]any{}
	for _, field := range []string{"id", "filename", "contentType", "size", "disposition", "contentId"} {
		if value, present := attachment[field]; present {
			summary[field] = value
		}
	}
	return summary
}

// AttachmentRoot answers where legacy path-backed attachments live.
func (AttachmentsRecords) AttachmentRoot() (string, error) {
	settings := Settings{}
	if root := settings.Value("ATTACHMENT_ROOT"); root != "" {
		return root, nil
	}
	if root := settings.Value("FYLO_ROOT"); root != "" {
		return filepath.Join(root, "attachments"), nil
	}
	return "", errors.New("FYLO_ROOT or legacy ATTACHMENT_ROOT is required to read a path-backed attachment")
}

// PersistOne stores one attachment's bytes in the attachment bucket and its
// record in the attachments collection, and answers the record with its
// docId. A record that cannot be stored takes its bucket file with it.
func (records AttachmentsRecords) PersistOne(database FyloDatabase, emailId string, domain string, attachment ParsedAttachment) (map[string]any, error) {
	random := RandomText{}
	id := random.Hex(16)
	filename := AttachmentPolicy{}.SanitizeAttachmentFilename(attachment.Filename, "attachment-"+id)
	content := records.ReadBytes(attachment)
	names := FyloCollections{}.Named()
	size := len(content)
	staged := IncomingFile{Prefix: "attachment-", Name: filename, Mode: 0o666}
	bucketId, err := staged.Stored(database.Root, content, func(path string) (string, error) {
		return database.PutFile(names.AttachmentFiles, FyloFile{Path: path, Key: records.BucketKey(emailId, id, filename), MaxBytes: &size, Meta: map[string]any{"domain": domain}})
	})
	if err != nil {
		return nil, err
	}
	record := map[string]any{
		"id": id, "emailId": emailId, "domain": domain, "filename": filename,
		"contentType": records.contentType(attachment.ContentType), "size": size,
		"disposition": TextValue{}.Or(attachment.Disposition, "attachment"),
		"bucketId": bucketId, "createdAt": random.Now(),
	}
	if contentId := (AttachmentPolicy{}).NormalizeAttachmentContentId(attachment.ContentId); contentId != "" {
		record["contentId"] = contentId
	}
	docId, err := database.PutData(names.Attachments, record)
	if err != nil {
		_ = database.DelDoc(names.AttachmentFiles, bucketId)
		return nil, err
	}
	stored := TextValue{}.Copy(record)
	stored["docId"] = docId
	return stored, nil
}

func (AttachmentsRecords) contentType(value string) string {
	if value == "" {
		value = "application/octet-stream"
	}
	kind, _, _ := strings.Cut(value, ";")
	return strings.ToLower(kind)
}

// BucketKey answers the bucket key of one attachment file.
func (AttachmentsRecords) BucketKey(emailId string, id string, filename string) string {
	return fmt.Sprintf("/emails/%s/%s-%s", RandomText{}.Digest([]byte(emailId)), id, filepath.Base(filename))
}

// EnsureInsideRoot refuses a target path outside root.
func (AttachmentsRecords) EnsureInsideRoot(root string, target string) error {
	resolvedRoot, _ := filepath.Abs(root)
	resolvedTarget, _ := filepath.Abs(target)
	relative, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil || strings.HasPrefix(relative, "..") || strings.HasPrefix(relative, "/") {
		return errors.New("Attachment path escapes storage root")
	}
	return nil
}

// ReadBytes answers an attachment's bytes: its content, or its decoded
// base64 text.
func (AttachmentsRecords) ReadBytes(attachment ParsedAttachment) []byte {
	if attachment.Content != nil {
		return attachment.Content
	}
	return Base64Text{}.Decoded(attachment.ContentBase64)
}

// DeleteRecord deletes one attachment record and its stored file. A legacy
// file that cannot be removed is reported and does not fail the deletion.
func (records AttachmentsRecords) DeleteRecord(database FyloDatabase, record map[string]any) error {
	text := TextValue{}
	docId := text.Of(record["docId"])
	if !text.Truthy(record["docId"]) {
		return errors.New("Attachment record is missing its document id")
	}
	names := FyloCollections{}.Named()
	if err := database.DelDoc(names.Attachments, docId); err != nil {
		return err
	}
	if text.Truthy(record["bucketId"]) {
		return database.DelDoc(names.AttachmentFiles, text.Of(record["bucketId"]))
	}
	if text.Truthy(record["storagePath"]) {
		if err := records.removeLegacy(text.Of(record["storagePath"])); err != nil {
			fmt.Fprintln(os.Stderr, "[attachments] delete legacy attachment file failed:", err)
		}
	}
	return nil
}

func (records AttachmentsRecords) removeLegacy(path string) error {
	root, err := records.AttachmentRoot()
	if err != nil {
		return err
	}
	if err := records.EnsureInsideRoot(root, path); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Grouped answers the summaries of the stored attachments of the allowed
// domains, grouped by email id.
func (records AttachmentsRecords) Grouped(stored []FyloRecord, allowedDomains []string) map[string][]map[string]any {
	text := TextValue{}
	grouped := map[string][]map[string]any{}
	for _, record := range stored {
		if !text.Includes(allowedDomains, record.Data["domain"]) {
			continue
		}
		emailId := text.Of(record.Data["emailId"])
		grouped[emailId] = append(grouped[emailId], records.ToSummary(record.Data))
	}
	return grouped
}

// ReadContent answers the bytes of one attachment, from the bucket or from
// a legacy file path inside the attachment root.
func (records AttachmentsRecords) ReadContent(database FyloDatabase, attachment map[string]any) ([]byte, error) {
	text := TextValue{}
	if text.Truthy(attachment["bucketId"]) {
		return database.GetFileBytes(FyloCollections{}.Named().AttachmentFiles, text.Of(attachment["bucketId"]))
	}
	if !text.Truthy(attachment["storagePath"]) {
		return nil, errors.New("Attachment has no stored content reference")
	}
	return records.ReadLegacy(text.Of(attachment["storagePath"]))
}

// ReadLegacy answers the bytes of a legacy attachment file inside the
// attachment root.
func (records AttachmentsRecords) ReadLegacy(path string) ([]byte, error) {
	root, err := records.AttachmentRoot()
	if err != nil {
		return nil, err
	}
	if err := records.EnsureInsideRoot(root, path); err != nil {
		return nil, err
	}
	return records.ReadFileBytes(path)
}

// ReadFileBytes answers the bytes of a legacy attachment file.
func (AttachmentsRecords) ReadFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}
