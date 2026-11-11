package compose

type Composer struct {
	Boundaries BoundarySource
}

func (composer Composer) Compose(message Message) ([]byte, error) {
	err := checker{}.check(message)
	if err != nil {
		return nil, err
	}
	content := bodyBuilder{boundaries: composer.boundarySource()}.build(message)
	headers := headerBuilder{}.build(message)
	return documentWriter{}.document(headers, content), nil
}

func (composer Composer) boundarySource() BoundarySource {
	if composer.Boundaries == nil {
		return RandomBoundaries{}
	}
	return composer.Boundaries
}
