package core

type InboundMail interface {
	Content(domainId string, inboundId string) (InboundContent, error)
	Original(domainId string, inboundId string) (Download, error)
}
