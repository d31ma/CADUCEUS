package startup

import (
	"strings"
	"unicode"
)

type SettingsReader struct {
	environment Environment
}

func NewSettingsReader(environment Environment) SettingsReader {
	return SettingsReader{environment: environment}
}

func (reader SettingsReader) Load() (Settings, error) {
	secret, err := reader.secret()
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		PublicAddress:     reader.environment.Text("MESAIL_REALTIME_LISTEN", reader.defaultPublicAddress()),
		ManagementAddress: reader.environment.Text("MESAIL_REALTIME_MANAGE_LISTEN", reader.defaultManagementAddress()),
		BackendUrl:        reader.environment.Text("MESAIL_REALTIME_BACKEND", reader.defaultBackendUrl()),
		Secret:            secret,
		Origins:           reader.origins(reader.environment.Optional("MESAIL_REALTIME_ORIGINS")),
	}, nil
}

func (reader SettingsReader) secret() (string, error) {
	value := reader.environment.Optional(reader.secretName())
	if len(value) < reader.minimumSecretLength() {
		return "", ShortSecret{Name: reader.secretName(), Minimum: reader.minimumSecretLength()}
	}
	return value, nil
}

func (SettingsReader) origins(text string) []string {
	return strings.FieldsFunc(text, func(character rune) bool {
		return character == ',' || unicode.IsSpace(character)
	})
}

func (SettingsReader) secretName() string               { return "MESAIL_REALTIME_GATEWAY_SECRET" }
func (SettingsReader) minimumSecretLength() int         { return 32 }
func (SettingsReader) defaultPublicAddress() string     { return "127.0.0.1:8116" }
func (SettingsReader) defaultManagementAddress() string { return "127.0.0.1:8117" }
func (SettingsReader) defaultBackendUrl() string        { return "http://127.0.0.1:8106" }
