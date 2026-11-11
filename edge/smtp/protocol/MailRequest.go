package protocol

import (
	"strconv"
	"strings"
)

type MailRequest struct {
	Sender string
	Size   int64
}

func (request MailRequest) WithParameters(fields []string) (MailRequest, error) {
	return request.applyAll(fields)
}

func (request MailRequest) applyAll(fields []string) (MailRequest, error) {
	for _, field := range fields {
		key, value, _ := strings.Cut(field, "=")
		updated, err := request.applierFor(key)(value)
		if err != nil {
			return MailRequest{}, err
		}
		request = updated
	}
	return request, nil
}

func (request MailRequest) applierFor(key string) func(string) (MailRequest, error) {
	if strings.EqualFold(key, request.sizeKey()) {
		return request.withSize
	}
	return request.unchanged
}

func (request MailRequest) withSize(value string) (MailRequest, error) {
	size, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return MailRequest{}, BadSyntax{}
	}
	request.Size = size
	return request, nil
}

func (request MailRequest) unchanged(_ string) (MailRequest, error) {
	return request, nil
}

func (MailRequest) sizeKey() string {
	return "SIZE"
}
