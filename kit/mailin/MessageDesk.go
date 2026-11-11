package mailin

import (
	"time"

	"mesail.dev/packages/blobs"
)

type MessageDesk struct {
	Callers  CallerGate
	Book     InboundBook
	Contents ContentShelf
	Blobs    blobs.Store
	LinkLife time.Duration
	Now      func() time.Time
}

func (desk MessageDesk) Original(authorization string, inboundId string) (Download, error) {
	stored, err := desk.message(authorization, inboundId)
	if err != nil {
		return Download{}, err
	}
	return desk.download(stored)
}

func (desk MessageDesk) Content(authorization string, inboundId string) (InboundContent, error) {
	stored, err := desk.message(authorization, inboundId)
	if err != nil {
		return InboundContent{}, err
	}
	return desk.Contents.ForMessage(stored)
}

func (desk MessageDesk) OriginalTerms() blobs.DownloadTerms {
	return blobs.DownloadTerms{FileName: "message.eml", MediaType: "message/rfc822"}
}

func (desk MessageDesk) message(authorization string, inboundId string) (InboundRaw, error) {
	caller, err := desk.Callers.Identify(authorization)
	if err != nil {
		return InboundRaw{}, Unauthenticated{}
	}
	return desk.Book.Find(caller.DomainId, inboundId)
}

func (desk MessageDesk) download(stored InboundRaw) (Download, error) {
	expiresAt := desk.Now().Add(desk.LinkLife).UTC()
	url, err := desk.Blobs.SignDownload(BlobKeys{}.FromPath(stored.RawBlobKey), desk.OriginalTerms(), expiresAt)
	return Download{Url: url, ExpiresAt: expiresAt.Format(time.RFC3339)}, desk.linkFailure(err)
}

func (MessageDesk) linkFailure(err error) error {
	if err == nil {
		return nil
	}
	return Unavailable{Operation: "download link"}
}
