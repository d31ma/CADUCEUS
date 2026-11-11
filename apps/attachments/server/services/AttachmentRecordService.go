import (
	"regexp"
	"strings"
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/collections"
	"mesail.dev/packages/ids"
)

// @Service
type AttachmentRecordService struct {
	// @Needs
	dataSource DataSource
}

func (service *AttachmentRecordService) Collection(domainId string) (string, error) {
	collection := collections.CollectionName{}.ForDomain("attachments", domainId)
	exists, err := service.dataSource.CollectionExists(collection)
	if err != nil {
		return "", service.failure("collection check", err)
	}
	return collection, service.failure("create collection", service.create(collection, exists))
}

func (service *AttachmentRecordService) Insert(collection string, record AttachmentRecord) (AttachmentRecord, error) {
	record.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	id, err := service.dataSource.Insert(collection, record)
	record.ID = id
	return record, service.failure("insert", err)
}

func (service *AttachmentRecordService) Lookup(domainId string, attachmentId string) (AttachmentRecord, error) {
	storedId, err := service.storedId(attachmentId)
	if err != nil {
		return AttachmentRecord{}, err
	}
	collection, err := service.Collection(domainId)
	if err != nil {
		return AttachmentRecord{}, err
	}
	record := AttachmentRecord{}
	exists, err := service.dataSource.FindById(collection, storedId, &record)
	if err != nil {
		return AttachmentRecord{}, service.failure("find", err)
	}
	record.ID = storedId
	return record, service.owned(record, exists, domainId)
}

func (service *AttachmentRecordService) SetState(record AttachmentRecord, state string) (AttachmentRecord, error) {
	collection, err := service.Collection(record.DomainId)
	if err != nil {
		return AttachmentRecord{}, err
	}
	_, err = service.dataSource.Patch(collection, record.ID, AttachmentStatePatch{State: state}, nil)
	record.State = state
	return record, service.failure("state", err)
}

func (service *AttachmentRecordService) View(record AttachmentRecord) Attachment {
	return Attachment{
		AttachmentId: service.WireId(record), FileName: record.FileName, MediaType: record.MediaType,
		Size: record.Size, Sha256: record.Sha256, State: record.State,
	}
}

func (service *AttachmentRecordService) WireId(record AttachmentRecord) string {
	return ids.WireId{}.Add(service.wirePrefix(), record.ID)
}

func (service *AttachmentRecordService) Key(record AttachmentRecord) blobs.Key {
	domainId, name, _ := strings.Cut(record.BlobKey, "/")
	return blobs.Key{DomainId: domainId, Name: name}
}

func (service *AttachmentRecordService) storedId(attachmentId string) (string, error) {
	storedId, failure := ids.WireId{}.Strip(service.wirePrefix(), attachmentId)
	if failure != nil || !regexp.MustCompile(service.storedIdPattern()).MatchString(storedId) {
		return "", &AttachmentNotFound{}
	}
	return storedId, nil
}

func (service *AttachmentRecordService) create(collection string, exists bool) error {
	if exists {
		return nil
	}
	return service.dataSource.CreateCollection(collection)
}

func (service *AttachmentRecordService) owned(record AttachmentRecord, exists bool, domainId string) error {
	if !exists || record.DomainId != domainId {
		return &AttachmentNotFound{}
	}
	return nil
}

func (service *AttachmentRecordService) failure(operation string, cause error) error {
	if cause == nil {
		return nil
	}
	return &StorageUnavailable{Operation: operation}
}

func (service *AttachmentRecordService) wirePrefix() string {
	return "att"
}

func (service *AttachmentRecordService) storedIdPattern() string {
	return `^[A-Za-z0-9_-]{1,64}$`
}
