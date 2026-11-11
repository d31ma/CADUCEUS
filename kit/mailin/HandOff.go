package mailin

import (
	"errors"

	"mesail.dev/packages/blobs"
)

type HandOff struct {
	Domains Domains
	Blobs   blobs.Store
}

func (handOff HandOff) Received(envelope Envelope, domainId string) (Envelope, error) {
	if !envelope.AuthResults.Handoff {
		return envelope, nil
	}
	return handOff.copied(envelope, domainId)
}

func (handOff HandOff) copied(envelope Envelope, domainId string) (Envelope, error) {
	err := handOff.checkSender(EnvelopeRules{}.DomainOf(envelope.EnvelopeFrom))
	if err != nil {
		return Envelope{}, err
	}
	target := BlobKeys{}.HandOffCopy(domainId, envelope.RawBlobKey)
	err = handOff.Blobs.Copy(BlobKeys{}.FromPath(envelope.RawBlobKey), target)
	if err != nil {
		return Envelope{}, handOff.copyFailure(err)
	}
	envelope.RawBlobKey = target.Path()
	return envelope, nil
}

func (handOff HandOff) checkSender(domainId string) error {
	domain, found, err := handOff.Domains.Domain(domainId)
	if err != nil {
		return Unavailable{Operation: "directory"}
	}
	return handOff.hosted(domain, found)
}

func (HandOff) hosted(domain Domain, found bool) error {
	if found && domain.IsActive() {
		return nil
	}
	return Invalid{Field: "envelopeFrom", Reason: "the sender domain is not hosted or not active"}
}

func (HandOff) copyFailure(failure error) error {
	missing := blobs.NotFound{}
	if errors.As(failure, &missing) {
		return Invalid{Field: "rawBlobKey", Reason: "no raw message is stored under this key"}
	}
	return Unavailable{Operation: "blob copy"}
}
