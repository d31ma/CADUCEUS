package dkim

import (
	"os"
	"regexp"
	"strings"
)

type EnvironmentSecretSource struct {
	Prefix string
}

func (environment EnvironmentSecretSource) Secret(name string) ([]byte, error) {
	return environment.lookup(name, environment.VariableName(name))
}

func (environment EnvironmentSecretSource) VariableName(name string) string {
	return environment.Prefix + environment.separators().ReplaceAllString(strings.ToUpper(name), "_")
}

func (EnvironmentSecretSource) separators() *regexp.Regexp {
	return regexp.MustCompile(`[^A-Z0-9]`)
}

func (EnvironmentSecretSource) lookup(name string, variable string) ([]byte, error) {
	value, found := os.LookupEnv(variable)
	if !found {
		return nil, MissingSecret{Name: name}
	}
	return []byte(value), nil
}
