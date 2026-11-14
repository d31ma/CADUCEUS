package startup

import (
	"net"
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
	keysUrl, err := reader.keysUrl()
	if err != nil {
		return Settings{}, err
	}
	publicAddresses, err := reader.publicAddresses()
	return Settings{
		HttpAddress:     reader.environment.Text("MESAIL_CALLS_LISTEN", reader.defaultHttpAddress()),
		MediaAddress:    reader.environment.Text("MESAIL_CALLS_MEDIA_LISTEN", reader.defaultMediaAddress()),
		PublicAddresses: publicAddresses,
		KeysUrl:         keysUrl,
	}, err
}

func (reader SettingsReader) keysUrl() (string, error) {
	value := reader.environment.Text(reader.keysName(), "")
	if value == "" {
		return "", MissingSetting{Name: reader.keysName()}
	}
	return value, nil
}

func (reader SettingsReader) publicAddresses() ([]string, error) {
	addresses := strings.FieldsFunc(reader.environment.Text(reader.publicName(), ""), reader.separator)
	return addresses, reader.firstInvalid(addresses)
}

func (reader SettingsReader) firstInvalid(addresses []string) error {
	for _, address := range addresses {
		err := reader.checked(address)
		if err != nil {
			return err
		}
	}
	return nil
}

func (reader SettingsReader) checked(address string) error {
	if net.ParseIP(address) == nil {
		return InvalidAddress{Name: reader.publicName(), Value: address}
	}
	return nil
}

func (SettingsReader) separator(character rune) bool {
	return character == ',' || unicode.IsSpace(character)
}

func (SettingsReader) keysName() string            { return "MESAIL_CALLS_KEYS_URL" }
func (SettingsReader) publicName() string          { return "MESAIL_CALLS_PUBLIC_IPS" }
func (SettingsReader) defaultHttpAddress() string  { return ":8120" }
func (SettingsReader) defaultMediaAddress() string { return ":8121" }
