package protocol

import "strings"

type Paths struct{}

func (paths Paths) Mail(argument string) (MailRequest, error) {
	rest, err := paths.after(argument, paths.fromPrefix())
	if err != nil {
		return MailRequest{}, err
	}
	sender, parameters, err := paths.split(rest)
	if err != nil {
		return MailRequest{}, err
	}
	return MailRequest{Sender: strings.ToLower(sender)}.WithParameters(strings.Fields(parameters))
}

func (paths Paths) Recipient(argument string) (string, error) {
	rest, err := paths.after(argument, paths.toPrefix())
	if err != nil {
		return "", err
	}
	address, _, err := paths.split(rest)
	if err != nil {
		return "", err
	}
	return paths.mailbox(strings.ToLower(address))
}

func (Paths) after(argument string, prefix string) (string, error) {
	if !strings.HasPrefix(strings.ToUpper(argument), prefix) {
		return "", BadSyntax{}
	}
	return strings.TrimSpace(argument[len(prefix):]), nil
}

func (Paths) split(rest string) (string, string, error) {
	inside, parameters, found := strings.Cut(strings.TrimPrefix(rest, "<"), ">")
	if !found || !strings.HasPrefix(rest, "<") {
		return "", "", BadSyntax{}
	}
	return strings.TrimSpace(inside), parameters, nil
}

func (Paths) mailbox(address string) (string, error) {
	local, domain, found := strings.Cut(address, "@")
	if !found || local == "" || domain == "" {
		return "", BadSyntax{}
	}
	return address, nil
}

func (Paths) fromPrefix() string {
	return "FROM:"
}

func (Paths) toPrefix() string {
	return "TO:"
}
