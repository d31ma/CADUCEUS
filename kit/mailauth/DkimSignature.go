package mailauth

import (
	"cmp"
	"encoding/base64"
	"slices"
	"strconv"
	"strings"
)

type dkimSignature struct {
	field        messageField
	domain       string
	selector     string
	algorithm    string
	headerMethod string
	bodyMethod   string
	headers      []string
	data         []byte
	bodyHash     []byte
	hasLength    bool
	length       int64
	hasExpiry    bool
	expires      int64
	canonical    dkimCanonical
}

func (dkimSignature) parse(field messageField) (dkimSignature, error) {
	tags, err := newTagList(field.value())
	signature := dkimSignature{field: field, domain: strings.ToLower(tags.compact("d")), selector: tags.compact("s")}
	if err != nil {
		return signature, err
	}
	return signature.fill(tags)
}

func (signature dkimSignature) fill(tags tagList) (dkimSignature, error) {
	err := signature.complete(tags)
	if err != nil {
		return signature, err
	}
	signature.algorithm = strings.ToLower(tags.compact("a"))
	signature.headers = signature.headerNames(tags.compact("h"))
	signature.headerMethod, signature.bodyMethod, err = signature.methods(tags.compact("c"))
	if err != nil {
		return signature, err
	}
	return signature.fillValues(tags)
}

func (signature dkimSignature) fillValues(tags tagList) (dkimSignature, error) {
	var err error
	signature.data, err = signature.decode(tags.compact("b"))
	if err != nil {
		return signature, err
	}
	signature.bodyHash, err = signature.decode(tags.compact("bh"))
	if err != nil {
		return signature, err
	}
	return signature.fillTimes(tags)
}

func (signature dkimSignature) fillTimes(tags tagList) (dkimSignature, error) {
	var err error
	signature.hasLength, signature.length, err = signature.number(tags, "l")
	if err != nil {
		return signature, err
	}
	signature.hasExpiry, signature.expires, err = signature.number(tags, "x")
	if err != nil {
		return signature, err
	}
	_, signedAt, err := signature.number(tags, "t")
	if err != nil {
		return signature, err
	}
	return signature, signature.consistent(tags, signedAt)
}

func (signature dkimSignature) consistent(tags tagList, signedAt int64) error {
	err := signature.signsFrom()
	if err != nil {
		return err
	}
	err = signature.identityMatches(strings.ToLower(tags.compact("i")))
	if err != nil {
		return err
	}
	return signature.ordered(signedAt)
}

func (signature dkimSignature) complete(tags tagList) error {
	missing := slices.ContainsFunc(signature.required(), tags.missing)
	if missing || tags.compact("v") != "1" {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}

func (dkimSignature) required() []string {
	return []string{"v", "a", "b", "bh", "d", "h", "s"}
}

func (dkimSignature) headerNames(text string) []string {
	names := strings.Split(text, ":")
	for index, name := range names {
		names[index] = strings.ToLower(name)
	}
	return names
}

func (signature dkimSignature) methods(text string) (string, string, error) {
	simple := signature.canonical.simple()
	header, body, _ := strings.Cut(cmp.Or(strings.ToLower(text), simple), "/")
	body = cmp.Or(body, simple)
	return header, body, signature.knownMethods(header, body)
}

func (signature dkimSignature) knownMethods(header string, body string) error {
	methods := signature.canonical.methods()
	if !slices.Contains(methods, header) || !slices.Contains(methods, body) {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}

func (dkimSignature) decode(text string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, failure{result: Outcomes{}.PermError()}
	}
	return data, nil
}

func (signature dkimSignature) number(tags tagList, name string) (bool, int64, error) {
	value, err := strconv.ParseUint(cmp.Or(tags.compact(name), "0"), signature.base(), signature.bits())
	if err != nil {
		return false, 0, failure{result: Outcomes{}.PermError()}
	}
	return tags.has(name), int64(value), nil
}

func (dkimSignature) base() int {
	return 10
}

func (dkimSignature) bits() int {
	return 63
}

func (signature dkimSignature) signsFrom() error {
	if !slices.Contains(signature.headers, "from") {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}

func (signature dkimSignature) identityMatches(identity string) error {
	identityDomain := identity[strings.LastIndex(identity, "@")+1:]
	if identity != "" && identityDomain != signature.domain && !strings.HasSuffix(identityDomain, "."+signature.domain) {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}

func (signature dkimSignature) ordered(signedAt int64) error {
	if signature.hasExpiry && signature.expires < signedAt {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}
