package main

import (
	"flag"
	"io"
)

type Arguments struct {
	Root string
	Base string
}

func (Arguments) Parse(values []string) (Arguments, error) {
	arguments := Arguments{}
	flags := flag.NewFlagSet("gateplan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&arguments.Root, "root", ".", "the repository root")
	flags.StringVar(&arguments.Base, "base", "origin/develop", "the branch the change is compared with")
	failure := flags.Parse(values)
	return arguments, failure
}
