package blobs

import (
	"net/url"
	"slices"
	"strings"
)

type AwsEncoding struct{}

func (encoding AwsEncoding) Escape(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func (encoding AwsEncoding) Query(query url.Values) string {
	pairs := encoding.pairs(query)
	slices.Sort(pairs)
	return strings.Join(pairs, "&")
}

func (encoding AwsEncoding) pairs(query url.Values) []string {
	pairs := []string{}
	for name, values := range query {
		pairs = append(pairs, encoding.joined(name, values)...)
	}
	return pairs
}

func (encoding AwsEncoding) joined(name string, values []string) []string {
	pairs := []string{}
	for _, value := range values {
		pairs = append(pairs, encoding.Escape(name)+"="+encoding.Escape(value))
	}
	return pairs
}
