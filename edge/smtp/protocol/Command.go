package protocol

import "strings"

type Command struct {
	Verb     string
	Argument string
}

func (Command) Parse(line string) Command {
	verb, argument, _ := strings.Cut(strings.TrimSpace(line), " ")
	return Command{Verb: strings.ToUpper(verb), Argument: strings.TrimSpace(argument)}
}
