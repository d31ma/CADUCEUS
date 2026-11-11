package mime

import (
	"io"
	stdmime "mime"
	"strings"
)

type encodedWords struct{}

func (words encodedWords) Decode(value string) string {
	decoded, err := words.decoder().DecodeHeader(value)
	return charsets{}.Valid(words.choose(decoded, value, err))
}

func (words encodedWords) decoder() *stdmime.WordDecoder {
	return &stdmime.WordDecoder{CharsetReader: words.charsetReader}
}

func (encodedWords) charsetReader(charset string, input io.Reader) (io.Reader, error) {
	content, err := io.ReadAll(input)
	return strings.NewReader(charsets{}.Decode(charset, content)), err
}

func (encodedWords) choose(decoded string, original string, err error) string {
	if err != nil {
		return original
	}
	return decoded
}
