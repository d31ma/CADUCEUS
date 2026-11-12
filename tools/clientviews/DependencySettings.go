package main

type DependencySettings struct {
	Check *bool `json:"check"`
}

func (settings DependencySettings) TurnedOff() bool {
	return settings.isFalse(settings.Check)
}

func (DependencySettings) isFalse(check *bool) bool {
	return check != nil && !*check
}
