package search

import "mesail.dev/packages/events"

type Indexer struct {
	Storage IndexStorage
	Builder DocumentBuilder
}

func (indexer Indexer) Handle(envelope events.Envelope) error {
	types := events.Types{}
	switch envelope.Type {
	case types.MessageCreated():
		return indexer.created(envelope)
	case types.MessageEdited():
		return indexer.edited(envelope)
	case types.MessageDeleted():
		return indexer.deleted(envelope)
	default:
		return nil
	}
}

func (indexer Indexer) created(envelope events.Envelope) error {
	message := events.MessageCreated{}
	err := indexer.payload(envelope, &message)
	if err != nil {
		return err
	}
	document, month, err := indexer.Builder.FromCreated(envelope.DomainId, message)
	if err != nil {
		return err
	}
	_, found, err := indexer.Storage.Location(envelope.DomainId, message.MessageId)
	if err != nil {
		return err
	}
	return indexer.addUnless(found, document, month)
}

func (indexer Indexer) addUnless(found bool, document Document, month string) error {
	if found {
		return nil
	}
	return indexer.Storage.Add(document, month)
}

func (indexer Indexer) edited(envelope events.Envelope) error {
	message := events.MessageEdited{}
	err := indexer.payload(envelope, &message)
	if err != nil {
		return err
	}
	location, _, err := indexer.Storage.Location(envelope.DomainId, message.MessageId)
	if err != nil {
		return err
	}
	return indexer.editWhen(indexer.Storage.Live(location), location, message)
}

func (indexer Indexer) editWhen(live bool, location Location, message events.MessageEdited) error {
	if !live {
		return nil
	}
	document, err := indexer.Storage.Find(location)
	if err != nil {
		return err
	}
	edited, err := indexer.Builder.Edited(document, message)
	if err != nil {
		return err
	}
	return indexer.Storage.Replace(location, edited)
}

func (indexer Indexer) deleted(envelope events.Envelope) error {
	message := events.MessageDeleted{}
	err := indexer.payload(envelope, &message)
	if err != nil {
		return err
	}
	location, _, err := indexer.Storage.Location(envelope.DomainId, message.MessageId)
	if err != nil {
		return err
	}
	err = indexer.eraseWhen(indexer.Storage.Live(location), location)
	if err != nil {
		return err
	}
	return indexer.Storage.Bury(envelope.DomainId, message.MessageId)
}

func (indexer Indexer) eraseWhen(indexed bool, location Location) error {
	if !indexed {
		return nil
	}
	document, err := indexer.Storage.Find(location)
	if err != nil {
		return err
	}
	return indexer.Storage.Replace(location, indexer.Builder.Tombstone(document))
}

func (Indexer) payload(envelope events.Envelope, target any) error {
	if (events.PayloadReader{}).Read(envelope, target) != nil {
		return InvalidRequest{Message: "the event payload does not match its type", Field: "payload"}
	}
	return nil
}
