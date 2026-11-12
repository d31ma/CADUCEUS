package oidc

type LabelExists struct{}

func (LabelExists) Error() string {
	return "another passkey of the person has this label"
}

func (LabelExists) PasskeyCode() string {
	return "label_exists"
}
