package mailin

import "mesail.dev/packages/collections"

type ContentShelf struct {
	Storage Storage
	Builder ContentBuilder
}

func (ContentShelf) Collection(domainId string) string {
	return collections.CollectionName{}.ForDomain("inbound-content", domainId)
}

func (shelf ContentShelf) ForMessage(stored InboundRaw) (InboundContent, error) {
	collection := shelf.Collection(stored.DomainId)
	err := shelf.Storage.Ensure(collection)
	if err != nil {
		return InboundContent{}, err
	}
	kept := []ContentRecord{}
	err = shelf.Storage.First(collection, map[string]any{"inboundId": stored.WireId()}, 1, &kept)
	if err != nil {
		return InboundContent{}, err
	}
	return shelf.keptOrBuilt(collection, stored, kept)
}

func (shelf ContentShelf) keptOrBuilt(collection string, stored InboundRaw, kept []ContentRecord) (InboundContent, error) {
	if len(kept) > 0 {
		return kept[0].Content()
	}
	return shelf.built(collection, stored)
}

func (shelf ContentShelf) built(collection string, stored InboundRaw) (InboundContent, error) {
	content, err := shelf.Builder.Build(stored)
	if err != nil {
		return InboundContent{}, err
	}
	_, err = shelf.Storage.Insert(collection, ContentRecord{}.From(stored.WireId(), content))
	return content, err
}
