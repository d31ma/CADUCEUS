package attachments

import (
	"crypto/rand"
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/oidc"
)

type Desk struct {
	Gate     CallerGate
	Book     RecordBook
	Store    blobs.Store
	Access   AccessRules
	Settings Settings
	Now      func() time.Time
	Failure  error
}

func (desk Desk) Create(authorization string, request UploadRequest) (Upload, error) {
	caller, err := desk.caller(authorization)
	if err != nil {
		return Upload{}, err
	}
	err = UploadRules{}.Check(request, desk.Settings.Limit.Bytes)
	if err != nil {
		return Upload{}, err
	}
	record, err := desk.Book.Insert(desk.pending(caller, request))
	if err != nil {
		return Upload{}, err
	}
	return desk.upload(record)
}

func (desk Desk) Complete(authorization string, attachmentId string) (View, error) {
	caller, err := desk.caller(authorization)
	if err != nil {
		return View{}, err
	}
	record, err := desk.Book.Lookup(caller.DomainId, attachmentId)
	if err != nil {
		return View{}, err
	}
	err = desk.checkUploader(caller, record)
	if err != nil {
		return View{}, err
	}
	return desk.completed(record)
}

func (desk Desk) Download(authorization string, attachmentId string) (Download, error) {
	caller, err := desk.caller(authorization)
	if err != nil {
		return Download{}, err
	}
	record, err := desk.Book.Lookup(caller.DomainId, attachmentId)
	if err != nil {
		return Download{}, err
	}
	err = desk.Access.Check(caller, record, ReadQuestion{AttachmentId: attachmentId, Authorization: authorization})
	if err != nil {
		return Download{}, err
	}
	err = desk.checkReady(record)
	if err != nil {
		return Download{}, err
	}
	return desk.download(record)
}

func (desk Desk) Meta(authorization string, attachmentId string) (View, error) {
	caller, err := desk.identify(authorization)
	if err != nil {
		return View{}, err
	}
	err = desk.checkService(caller)
	if err != nil {
		return View{}, err
	}
	record, err := desk.Book.Lookup(caller.DomainId, attachmentId)
	if err != nil {
		return View{}, err
	}
	return desk.Book.View(record), nil
}

func (desk Desk) caller(authorization string) (oidc.Caller, error) {
	caller, err := desk.identify(authorization)
	if err != nil {
		return oidc.Caller{}, err
	}
	return caller, desk.Failure
}

func (desk Desk) identify(authorization string) (oidc.Caller, error) {
	caller, err := desk.Gate.Identify(authorization)
	if err != nil {
		return oidc.Caller{}, Unauthenticated{}
	}
	return caller, nil
}

func (desk Desk) pending(caller oidc.Caller, request UploadRequest) Record {
	key := blobs.Key{DomainId: caller.DomainId, Name: rand.Text()}
	return Record{
		DomainId: caller.DomainId, UploaderId: caller.MemberId, FileName: request.FileName, MediaType: request.MediaType,
		Size: request.Size, Sha256: request.Sha256, State: States{}.Pending(), BlobKey: key.Path(),
	}
}

func (desk Desk) upload(record Record) (Upload, error) {
	expiresAt := desk.expiry()
	terms := blobs.UploadTerms{MediaType: record.MediaType, Size: record.Size}
	uploadUrl, err := desk.Store.SignUpload(desk.Book.Key(record), terms, expiresAt)
	if err != nil {
		return Upload{}, Unavailable{Operation: "sign upload"}
	}
	return Upload{AttachmentId: desk.Book.WireId(record), UploadUrl: uploadUrl, ExpiresAt: expiresAt.Format(time.RFC3339)}, nil
}

func (Desk) checkUploader(caller oidc.Caller, record Record) error {
	if caller.MemberId != record.UploaderId {
		return Missing{}
	}
	return nil
}

func (desk Desk) checkService(caller oidc.Caller) error {
	if !desk.Access.isService(caller) {
		return Forbidden{}
	}
	return nil
}

func (desk Desk) completed(record Record) (View, error) {
	if record.State != (States{}).Pending() {
		return desk.Book.View(record), nil
	}
	return desk.verified(record)
}

func (desk Desk) verified(record Record) (View, error) {
	state, err := Verifier{}.Verify(desk.Store, desk.Book.Key(record), record)
	if err != nil {
		return View{}, err
	}
	saved, err := desk.Book.SetState(record, state)
	return desk.Book.View(saved), err
}

func (Desk) checkReady(record Record) error {
	if record.State != (States{}).Ready() {
		return Conflict{Field: "state", Reason: "The attachment is not ready to download."}
	}
	return nil
}

func (desk Desk) download(record Record) (Download, error) {
	expiresAt := desk.expiry()
	terms := blobs.DownloadTerms{FileName: record.FileName, MediaType: record.MediaType}
	downloadUrl, err := desk.Store.SignDownload(desk.Book.Key(record), terms, expiresAt)
	if err != nil {
		return Download{}, Unavailable{Operation: "sign download"}
	}
	return Download{Url: downloadUrl, ExpiresAt: expiresAt.Format(time.RFC3339)}, nil
}

func (desk Desk) expiry() time.Time {
	return desk.Now().UTC().Add(time.Duration(desk.Settings.Link.Seconds) * time.Second).Truncate(time.Second)
}
