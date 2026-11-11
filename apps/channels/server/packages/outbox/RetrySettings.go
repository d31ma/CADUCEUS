package outbox

type RetrySettings struct {
	BaseDelaySeconds int `json:"baseDelaySeconds"`
	MaxDelaySeconds  int `json:"maxDelaySeconds"`
	MaxAttempts      int `json:"maxAttempts"`
}

func (settings RetrySettings) withDefaults() RetrySettings {
	return RetrySettings{
		BaseDelaySeconds: settings.orDefault(settings.BaseDelaySeconds, settings.defaultBaseDelaySeconds()),
		MaxDelaySeconds:  settings.orDefault(settings.MaxDelaySeconds, settings.defaultMaxDelaySeconds()),
		MaxAttempts:      settings.orDefault(settings.MaxAttempts, settings.defaultMaxAttempts()),
	}
}

func (RetrySettings) orDefault(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func (RetrySettings) defaultBaseDelaySeconds() int {
	return 5
}

func (RetrySettings) defaultMaxDelaySeconds() int {
	return 900
}

func (RetrySettings) defaultMaxAttempts() int {
	return 12
}
