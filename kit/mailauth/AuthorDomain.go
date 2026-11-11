package mailauth

import (
	"io"
	"mime"
	"net/mail"
	"slices"
	"strings"
)

type authorDomain struct{}

func (author authorDomain) find(fields []messageField) (string, error) {
	froms := slices.DeleteFunc(slices.Clone(fields), author.notFrom)
	err := author.single(len(froms))
	if err != nil {
		return "", err
	}
	addresses, err := author.parse(froms[0])
	if err != nil {
		return "", err
	}
	return author.domain(addresses)
}

func (author authorDomain) notFrom(field messageField) bool {
	return !field.named("From")
}

func (authorDomain) single(count int) error {
	switch {
	case count == 0:
		return failure{result: Outcomes{}.None()}
	case count > 1:
		return failure{result: Outcomes{}.PermError()}
	default:
		return nil
	}
}

func (author authorDomain) parse(field messageField) ([]*mail.Address, error) {
	parser := mail.AddressParser{WordDecoder: &mime.WordDecoder{CharsetReader: author.anyCharset}}
	addresses, err := parser.ParseList(strings.ReplaceAll(field.value(), "\r\n", ""))
	if err != nil {
		return nil, failure{result: Outcomes{}.PermError()}
	}
	return addresses, nil
}

func (authorDomain) anyCharset(charset string, input io.Reader) (io.Reader, error) {
	return input, nil
}

func (authorDomain) domain(addresses []*mail.Address) (string, error) {
	if len(addresses) != 1 {
		return "", failure{result: Outcomes{}.PermError()}
	}
	address := addresses[0].Address
	return strings.ToLower(strings.TrimSuffix(address[strings.LastIndex(address, "@")+1:], ".")), nil
}
