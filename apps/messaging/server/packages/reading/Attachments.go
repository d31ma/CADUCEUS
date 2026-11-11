package reading

import (
	"cmp"
	"errors"
	"slices"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type Attachments struct {
	access  core.Access
	storage storage
}

func NewAttachments(stores core.Stores) Attachments {
	return Attachments{access: core.NewAccess(stores), storage: storage{stores: stores}}
}

func (attachments Attachments) Access(caller oidc.Caller, attachmentId string) error {
	uses, err := attachments.storage.attachmentUses(caller.DomainId, attachmentId)
	if err != nil {
		return err
	}
	return attachments.verdict(attachments.outcomes(caller, uses))
}

func (attachments Attachments) outcomes(caller oidc.Caller, uses []core.AttachmentUse) []error {
	outcomes := []error{}
	for _, use := range uses {
		outcomes = append(outcomes, attachments.readable(caller, use))
	}
	return outcomes
}

func (attachments Attachments) readable(caller oidc.Caller, use core.AttachmentUse) error {
	message, _, err := attachments.access.Message(caller, use.MessageId)
	if err != nil {
		return err
	}
	return attachments.carried(message, use.AttachmentId)
}

func (attachments Attachments) carried(message core.Message, attachmentId string) error {
	if slices.Contains(message.AttachmentIds, attachmentId) {
		return nil
	}
	return attachments.missing()
}

func (attachments Attachments) verdict(outcomes []error) error {
	if slices.Contains(outcomes, nil) {
		return nil
	}
	return cmp.Or(errors.Join(slices.DeleteFunc(outcomes, attachments.notFound)...), attachments.missing())
}

func (Attachments) notFound(failure error) bool {
	return errors.As(failure, new(*core.NotFound))
}

func (Attachments) missing() error {
	return &core.NotFound{Message: "no such attachment"}
}
