package mime

import "net/textproto"

type headerFields struct{}

func (headerFields) From(header textproto.MIMEHeader) Headers {
	addresses := addressList{}
	ids := messageIds{}
	return Headers{
		MessageId:  ids.First(header.Get("Message-Id")),
		InReplyTo:  ids.First(header.Get("In-Reply-To")),
		References: ids.All(header.Get("References")),
		Subject:    encodedWords{}.Decode(header.Get("Subject")),
		From:       addresses.First(header.Get("From")),
		To:         addresses.All(header.Get("To")),
		Cc:         addresses.All(header.Get("Cc")),
		Date:       charsets{}.Valid(header.Get("Date")),
	}
}
