package recipients

type Classifier struct {
	kinds Kinds
}

func (classifier Classifier) Kind(destination Destination) Kind {
	local := classifier.local(destination.LocalKind)
	remote := classifier.remote(destination.Hosted)
	return classifier.pick(classifier.sameDomain(destination), local, remote)
}

func (classifier Classifier) sameDomain(destination Destination) bool {
	return destination.CallerDomainId == destination.AddressDomainId
}

func (classifier Classifier) pick(sameDomain bool, local Kind, remote Kind) Kind {
	if sameDomain {
		return local
	}
	return remote
}

func (classifier Classifier) local(kind Kind) Kind {
	if kind == classifier.kinds.Member() || kind == classifier.kinds.Channel() {
		return kind
	}
	return classifier.kinds.External()
}

func (classifier Classifier) remote(hosted bool) Kind {
	if hosted {
		return classifier.kinds.OtherDomain()
	}
	return classifier.kinds.External()
}
