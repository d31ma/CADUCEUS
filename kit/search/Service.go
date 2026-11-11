package search

import (
	"net/url"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

type Service struct {
	Callers  Callers
	Filters  Filters
	Queries  Queries
	Receiver events.Receiver
}

func NewService(documents Documents, reader Reader, identifier Identifier) Service {
	store := DocumentStore{Documents: documents}
	storage := IndexStorage{Store: store}
	builder := DocumentBuilder{}
	return Service{
		Callers:  Callers{Identifier: identifier},
		Filters:  Filters{Builder: builder},
		Queries:  Queries{Storage: storage, Reader: reader},
		Receiver: events.NewReceiver("search", SeenEvents{Store: store}, Indexer{Storage: storage, Builder: builder}),
	}
}

func (service Service) Search(authorization string, values url.Values) (Page, error) {
	caller, err := service.Callers.Member(authorization)
	if err != nil {
		return Page{}, err
	}
	filter, err := service.Filters.Parse(values)
	if err != nil {
		return Page{}, err
	}
	return service.Queries.Search(caller.DomainId, authorization, filter)
}

func (service Service) Receive(caller oidc.Caller, envelope events.Envelope) (events.Receipt, error) {
	receipt, err := service.Receiver.Receive(caller, envelope)
	return receipt, EventFailures{}.Translated(err)
}

func (service Service) SearchInto(authorization string, values url.Values, target any) error {
	page, err := service.Search(authorization, values)
	if err != nil {
		return err
	}
	return Json{}.Copy(page, target)
}

func (service Service) ReceiveInto(caller oidc.Caller, envelope any, target any) error {
	decoded := events.Envelope{}
	err := Json{}.Copy(envelope, &decoded)
	if err != nil {
		return err
	}
	receipt, err := service.Receive(caller, decoded)
	if err != nil {
		return err
	}
	return Json{}.Copy(receipt, target)
}
