package mime

import "mime/multipart"

type walker struct {
	parser  Parser
	message Message
	parts   int
	hasText bool
	hasHtml bool
}

func newWalker(parser Parser, headers Headers) *walker {
	return &walker{parser: parser, message: Message{Headers: headers}}
}

func (walk *walker) walk(part entity) error {
	err := walk.admit(part.depth)
	if err != nil {
		return err
	}
	return walk.handlers()[part.isMultipart()](part)
}

func (walk *walker) result(err error) (Message, error) {
	if err != nil {
		return Message{}, err
	}
	return walk.message, nil
}

func (walk *walker) admit(depth int) error {
	walk.parts++
	return walk.firstError(walk.parser.depthLimit(depth), walk.parser.partLimit(walk.parts))
}

func (walk *walker) firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (walk *walker) handlers() map[bool]func(entity) error {
	return map[bool]func(entity) error{true: walk.children, false: walk.collect}
}

func (walk *walker) children(part entity) error {
	reader := multipart.NewReader(part.body, part.boundary())
	for {
		child, err := reader.NextRawPart()
		if err != nil {
			return nil
		}
		err = walk.walk(newEntity(child.Header, child, part.depth+1))
		if err != nil {
			return err
		}
	}
}

func (walk *walker) collect(part entity) error {
	store, known := walk.storers()[part.bodyKind()]
	walk.pick(store, known)(part, part.content())
	return nil
}

func (walk *walker) storers() map[string]func(entity, []byte) {
	return map[string]func(entity, []byte){entity{}.plainText(): walk.storeText, "text/html": walk.storeHtml}
}

func (walk *walker) pick(store func(entity, []byte), known bool) func(entity, []byte) {
	if known {
		return store
	}
	return walk.attach
}

func (walk *walker) storeText(part entity, content []byte) {
	walk.storeBody(part, content, &walk.hasText, &walk.message.Text)
}

func (walk *walker) storeHtml(part entity, content []byte) {
	walk.storeBody(part, content, &walk.hasHtml, &walk.message.Html)
}

func (walk *walker) storeBody(part entity, content []byte, filled *bool, target *string) {
	if *filled {
		walk.attach(part, content)
		return
	}
	*filled = true
	*target = charsets{}.Decode(part.charset(), content)
}

func (walk *walker) attach(part entity, content []byte) {
	attachment := Attachment{
		FileName:  part.fileName(),
		MediaType: part.mediaType,
		ContentId: part.contentId(),
		Inline:    part.inline(),
		Size:      int64(len(content)),
		Content:   content,
	}
	walk.message.Attachments = append(walk.message.Attachments, attachment)
}
