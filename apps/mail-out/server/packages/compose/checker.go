package compose

import "strings"

type checker struct {
	names fieldNames
}

func (checker checker) check(message Message) error {
	return checker.firstError([]error{
		checker.missingFrom(message.From),
		checker.missingRecipients(message),
		checker.firstUnsafe(checker.fields(message)),
	})
}

func (checker) firstError(errors []error) error {
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}

func (checker checker) missingFrom(from Address) error {
	if from.Email == "" {
		return MissingField{Field: checker.names.From()}
	}
	return nil
}

func (checker checker) missingRecipients(message Message) error {
	if len(message.To)+len(message.Cc) == 0 {
		return MissingField{Field: checker.names.To()}
	}
	return nil
}

func (checker) firstUnsafe(fields []headerField) error {
	for _, field := range fields {
		if strings.ContainsAny(field.value, limits{}.UnsafeCharacters()) {
			return UnsafeHeaderValue{Field: field.name}
		}
	}
	return nil
}

func (checker checker) fields(message Message) []headerField {
	names := checker.names
	fields := checker.addressFields(names.From(), []Address{message.From})
	fields = append(fields, checker.addressFields(names.To(), message.To)...)
	fields = append(fields, checker.addressFields(names.Cc(), message.Cc)...)
	fields = append(fields, checker.addressFields(names.ReplyTo(), message.ReplyTo)...)
	fields = append(fields,
		headerField{name: names.Subject(), value: message.Subject},
		headerField{name: names.MessageId(), value: message.MessageId},
		headerField{name: names.InReplyTo(), value: message.InReplyTo},
	)
	fields = append(fields, checker.textFields(names.References(), message.References)...)
	return append(fields, checker.attachmentFields(message.Attachments)...)
}

func (checker) addressFields(name string, addresses []Address) []headerField {
	fields := []headerField{}
	for _, address := range addresses {
		fields = append(fields, headerField{name: name, value: address.Name + address.Email})
	}
	return fields
}

func (checker) textFields(name string, values []string) []headerField {
	fields := []headerField{}
	for _, value := range values {
		fields = append(fields, headerField{name: name, value: value})
	}
	return fields
}

func (checker checker) attachmentFields(attachments []Attachment) []headerField {
	fields := []headerField{}
	for _, attachment := range attachments {
		fields = append(fields, headerField{name: checker.names.Attachment(), value: attachment.FileName + attachment.ContentType})
	}
	return fields
}
