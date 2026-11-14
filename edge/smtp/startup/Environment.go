package startup

import (
	"os"
	"slices"
	"strings"
)

type Environment struct {
	lookup func(name string) (string, bool)
	names  func() []string
}

func NewEnvironment(lookup func(name string) (string, bool), names func() []string) Environment {
	return Environment{lookup: lookup, names: names}
}

func NewProcessEnvironment() Environment {
	return NewEnvironment(os.LookupEnv, Environment{}.processNames)
}

func (environment Environment) Optional(name string) string {
	return environment.Text(name, "")
}

func (environment Environment) Text(name string, fallback string) string {
	value, _ := environment.lookup(name)
	return environment.orFallback(value, fallback)
}

func (environment Environment) NamesStartingWith(prefix string) []string {
	found := environment.matching(environment.names(), prefix)
	slices.Sort(found)
	return found
}

func (Environment) orFallback(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (environment Environment) matching(names []string, prefix string) []string {
	found := []string{}
	for _, name := range names {
		found = environment.withMatch(found, name, prefix)
	}
	return found
}

func (Environment) withMatch(found []string, name string, prefix string) []string {
	if !strings.HasPrefix(name, prefix) {
		return found
	}
	return append(found, name)
}

func (environment Environment) processNames() []string {
	names := []string{}
	for _, entry := range os.Environ() {
		names = append(names, environment.nameOf(entry))
	}
	return names
}

func (Environment) nameOf(entry string) string {
	name, _, _ := strings.Cut(entry, "=")
	return name
}
