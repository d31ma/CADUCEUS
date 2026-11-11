package startup

type Environment struct {
	lookup func(name string) (string, bool)
}

func NewEnvironment(lookup func(name string) (string, bool)) Environment {
	return Environment{lookup: lookup}
}

func (environment Environment) Optional(name string) string {
	return environment.Text(name, "")
}

func (environment Environment) Text(name string, fallback string) string {
	value, _ := environment.lookup(name)
	return environment.orFallback(value, fallback)
}

func (Environment) orFallback(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
