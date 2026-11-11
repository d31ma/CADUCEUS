package addresses

import "strings"

type LocalPart struct{}

func (part LocalPart) Normalized(input string) (string, error) {
	value, err := nameText{}.lowered(NameKinds{}.LocalPart(), input)
	if err != nil {
		return "", err
	}
	err = part.check(NameKinds{}.LocalPart(), value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (part LocalPart) check(kind string, value string) error {
	err := part.checkLength(kind, value)
	if err != nil {
		return err
	}
	err = part.checkCharacters(kind, value)
	if err != nil {
		return err
	}
	err = part.checkEnds(kind, value)
	if err != nil {
		return err
	}
	return part.checkDots(kind, value)
}

func (LocalPart) maximumLength() int {
	return 64
}

func (LocalPart) allowedCharacters() string {
	return "abcdefghijklmnopqrstuvwxyz0123456789._-"
}

func (LocalPart) punctuation() string {
	return "._-"
}

func (LocalPart) twoDots() string {
	return ".."
}

func (part LocalPart) checkLength(kind string, value string) error {
	if value == "" || len(value) > part.maximumLength() {
		return InvalidName{Kind: kind, Reason: "must be 1 to 64 characters"}
	}
	return nil
}

func (part LocalPart) checkCharacters(kind string, value string) error {
	if strings.ContainsFunc(value, part.notAllowed) {
		return InvalidName{Kind: kind, Reason: "may only hold a-z, 0-9, ., _ and -"}
	}
	return nil
}

func (part LocalPart) notAllowed(character rune) bool {
	return !strings.ContainsRune(part.allowedCharacters(), character)
}

func (part LocalPart) checkEnds(kind string, value string) error {
	if strings.Trim(value, part.punctuation()) != value {
		return InvalidName{Kind: kind, Reason: "must start and end with a letter or digit"}
	}
	return nil
}

func (part LocalPart) checkDots(kind string, value string) error {
	if strings.Contains(value, part.twoDots()) {
		return InvalidName{Kind: kind, Reason: "must not hold two dots in a row"}
	}
	return nil
}
