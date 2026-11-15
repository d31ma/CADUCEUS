package backups

import (
	"cmp"
	"time"

	"mesail.dev/packages/attachments"
	"mesail.dev/packages/blobs"
	"mesail.dev/packages/oidc"
)

type Desk struct {
	Gate     attachments.CallerGate
	Shelf    Shelf
	Store    blobs.Store
	Settings Settings
	Now      func() time.Time
	Failure  error
}

func (desk Desk) Mine(authorization string) (Backup, error) {
	record, err := desk.current(authorization)
	return desk.view(record), err
}

func (desk Desk) Start(authorization string, request StartRequest) (Upload, error) {
	caller, err := desk.caller(authorization)
	if err != nil {
		return Upload{}, err
	}
	err = StartRules{}.Check(request, desk.Settings.Limit.Bytes)
	if err != nil {
		return Upload{}, err
	}
	record, err := desk.mine(caller)
	if err != nil {
		return Upload{}, err
	}
	err = desk.checkExpected(record, request.ExpectedVersion)
	if err != nil {
		return Upload{}, err
	}
	return desk.started(record, request)
}

func (desk Desk) Complete(authorization string, version int64) (Backup, error) {
	caller, err := desk.caller(authorization)
	if err != nil {
		return Backup{}, err
	}
	record, err := desk.mine(caller)
	if err != nil {
		return Backup{}, err
	}
	return desk.finish(record, version)
}

func (desk Desk) Download(authorization string) (Download, error) {
	record, err := desk.current(authorization)
	if err != nil {
		return Download{}, err
	}
	return desk.download(record)
}

func (desk Desk) Delete(authorization string) error {
	caller, err := desk.caller(authorization)
	if err != nil {
		return err
	}
	return desk.Remove(caller.DomainId, caller.MemberId)
}

func (desk Desk) Remove(domainId string, personId string) error {
	err := desk.Failure
	if err != nil {
		return err
	}
	record, found, err := desk.Shelf.Find(domainId, personId)
	if err != nil {
		return err
	}
	return desk.removeWhenFound(record, found)
}

func (desk Desk) caller(authorization string) (oidc.Caller, error) {
	caller, err := desk.Gate.Identify(authorization)
	if err != nil {
		return oidc.Caller{}, attachments.Unauthenticated{}
	}
	return caller, desk.Failure
}

func (desk Desk) mine(caller oidc.Caller) (Record, error) {
	record, found, err := desk.Shelf.Find(caller.DomainId, caller.MemberId)
	return desk.orNew(record, found, caller), err
}

func (Desk) orNew(record Record, found bool, caller oidc.Caller) Record {
	if found {
		return record
	}
	return Record{DomainId: caller.DomainId, PersonId: caller.MemberId, KeySlots: []string{}, Pending: Pending{KeySlots: []string{}}}
}

func (desk Desk) current(authorization string) (Record, error) {
	caller, err := desk.caller(authorization)
	if err != nil {
		return Record{}, err
	}
	record, err := desk.mine(caller)
	return record, cmp.Or(err, desk.requireCurrent(record))
}

func (Desk) requireCurrent(record Record) error {
	if record.Version == 0 {
		return Missing{}
	}
	return nil
}

func (Desk) checkExpected(record Record, expected int64) error {
	if record.Version != expected {
		return Conflict{}.Replaced()
	}
	return nil
}

func (desk Desk) started(record Record, request StartRequest) (Upload, error) {
	version := record.Version + 1
	key := BlobNames{}.New(record.DomainId, record.PersonId, version)
	expiresAt := desk.expiry()
	uploadUrl, err := desk.Store.SignUpload(key, blobs.UploadTerms{MediaType: Limits{}.MediaType(), Size: request.Size}, expiresAt)
	if err != nil {
		return Upload{}, attachments.Unavailable{Operation: "sign backup upload"}
	}
	replaced := record.Pending.Blob
	record.Pending = desk.pending(version, key, request, expiresAt)
	_, err = desk.Shelf.Save(record)
	if err != nil {
		return Upload{}, err
	}
	desk.discard(replaced)
	return Upload{Version: version, UploadUrl: uploadUrl, ExpiresAt: expiresAt.Format(time.RFC3339)}, nil
}

func (desk Desk) pending(version int64, key blobs.Key, request StartRequest, expiresAt time.Time) Pending {
	completeBy := expiresAt.Add(desk.linkLife()).Format(time.RFC3339)
	return Pending{
		Version: version, Blob: key.Path(), Size: request.Size, Sha256: request.Sha256,
		KeySlots: SlotText{}.Encode(request.KeySlots), DeviceId: request.DeviceId, CompleteBy: completeBy,
	}
}

func (desk Desk) finish(record Record, version int64) (Backup, error) {
	if !desk.waiting(record, version) {
		return Backup{}, desk.notWaiting(record, version)
	}
	return desk.promoted(record)
}

func (desk Desk) waiting(record Record, version int64) bool {
	return version > 0 && record.Pending.Version == version && desk.inTime(record.Pending)
}

func (desk Desk) inTime(pending Pending) bool {
	completeBy, err := time.Parse(time.RFC3339, pending.CompleteBy)
	return err == nil && !desk.Now().After(completeBy)
}

func (Desk) notWaiting(record Record, version int64) error {
	if version <= record.Version {
		return Conflict{}.Replaced()
	}
	return Conflict{}.NotStarted()
}

func (desk Desk) promoted(record Record) (Backup, error) {
	err := ByteCheck{}.Check(desk.Store, record.Pending)
	if err != nil {
		return Backup{}, err
	}
	replaced := record.Blob
	saved, err := desk.Shelf.Save(desk.fromPending(record))
	if err != nil {
		return Backup{}, err
	}
	desk.discard(replaced)
	return desk.view(saved), nil
}

func (desk Desk) fromPending(record Record) Record {
	pending := record.Pending
	record.Version, record.Blob, record.Size, record.Sha256 = pending.Version, pending.Blob, pending.Size, pending.Sha256
	record.KeySlots, record.UpdatedByDeviceId = pending.KeySlots, pending.DeviceId
	record.UpdatedAt = desk.Now().UTC().Format(time.RFC3339)
	record.Pending = Pending{KeySlots: []string{}}
	return record
}

func (desk Desk) download(record Record) (Download, error) {
	expiresAt := desk.expiry()
	terms := blobs.DownloadTerms{FileName: Limits{}.FileName(), MediaType: Limits{}.MediaType()}
	url, err := desk.Store.SignDownload(BlobNames{}.Key(record.Blob), terms, expiresAt)
	if err != nil {
		return Download{}, attachments.Unavailable{Operation: "sign backup download"}
	}
	return Download{Url: url, ExpiresAt: expiresAt.Format(time.RFC3339)}, nil
}

func (desk Desk) removeWhenFound(record Record, found bool) error {
	if !found {
		return nil
	}
	return desk.removed(record)
}

func (desk Desk) removed(record Record) error {
	err := cmp.Or(desk.deleteBlob(record.Blob), desk.deleteBlob(record.Pending.Blob))
	if err != nil {
		return attachments.Unavailable{Operation: "backup delete bytes"}
	}
	return desk.Shelf.Delete(record)
}

func (desk Desk) discard(path string) {
	desk.deleteBlob(path)
}

func (desk Desk) deleteBlob(path string) error {
	if path == "" {
		return nil
	}
	return desk.Store.Delete(BlobNames{}.Key(path))
}

func (Desk) view(record Record) Backup {
	return Backup{
		Version: record.Version, Size: record.Size, Sha256: record.Sha256, KeySlots: SlotText{}.Decode(record.KeySlots),
		UpdatedAt: record.UpdatedAt, UpdatedByDeviceId: record.UpdatedByDeviceId,
	}
}

func (desk Desk) expiry() time.Time {
	return desk.Now().UTC().Add(desk.linkLife()).Truncate(time.Second)
}

func (desk Desk) linkLife() time.Duration {
	return time.Duration(desk.Settings.Link.Seconds) * time.Second
}
