package reading

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type Sources struct {
	access  core.Access
	storage storage
	inbound core.InboundMail
}

func NewSources(stores core.Stores, ports core.Ports) Sources {
	return Sources{access: core.NewAccess(stores), storage: storage{stores: stores}, inbound: ports.Inbound}
}

func (sources Sources) Source(caller oidc.Caller, messageId string) (core.MessageSource, error) {
	month, storedId, err := core.MessageIds{}.Parse(messageId)
	if err != nil {
		return core.MessageSource{}, err
	}
	message, found, err := sources.storage.message(caller.DomainId, month, storedId)
	if err != nil {
		return core.MessageSource{}, err
	}
	err = sources.present(message, found, caller.DomainId)
	if err != nil {
		return core.MessageSource{}, err
	}
	return core.MessageSource{
		MessageId:     core.MessageIds{}.Wire(message.Month, message.Id),
		DomainId:      message.DomainId,
		Body:          message.Body,
		AttachmentIds: append([]string{}, message.AttachmentIds...),
	}, nil
}

func (sources Sources) Original(caller oidc.Caller, messageId string) (core.Download, error) {
	message, _, err := sources.access.Message(caller, messageId)
	if err != nil {
		return core.Download{}, err
	}
	inboundId, err := sources.inboundIdOf(message)
	if err != nil {
		return core.Download{}, err
	}
	return sources.inbound.Original(message.DomainId, inboundId)
}

func (Sources) present(message core.Message, found bool, domainId string) error {
	if !found || message.DomainId != domainId {
		return refusals{}.noMessage()
	}
	return nil
}

func (Sources) inboundIdOf(message core.Message) (string, error) {
	if message.Email.InboundId == "" {
		return "", refusals{}.noMessage()
	}
	return message.Email.InboundId, nil
}
