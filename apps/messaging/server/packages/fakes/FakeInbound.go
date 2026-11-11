package fakes

import "mesail.dev/packages/core"

type FakeInbound struct {
	Contents  map[string]core.InboundContent
	Originals map[string]core.Download
	Failure   error
}

func NewFakeInbound() *FakeInbound {
	return &FakeInbound{Contents: map[string]core.InboundContent{}, Originals: map[string]core.Download{}}
}

func (inbound *FakeInbound) Content(domainId string, inboundId string) (core.InboundContent, error) {
	return inbound.Contents[domainId+"/"+inboundId], inbound.Failure
}

func (inbound *FakeInbound) Original(domainId string, inboundId string) (core.Download, error) {
	download, found := inbound.Originals[domainId+"/"+inboundId]
	return download, inbound.missing(found)
}

func (inbound *FakeInbound) missing(found bool) error {
	if found {
		return inbound.Failure
	}
	return &core.NotFound{Message: "no such original"}
}
