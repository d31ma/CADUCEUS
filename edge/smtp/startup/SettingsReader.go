package startup

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"mesail.dev/edge/smtp/adapters"
	"mesail.dev/edge/smtp/protocol"
)

type SettingsReader struct {
	environment Environment
	problems    []error
}

func NewSettingsReader(environment Environment) *SettingsReader {
	return &SettingsReader{environment: environment}
}

func (reader *SettingsReader) Load() (ListenerSettings, error) {
	settings := ListenerSettings{
		Address:         net.JoinHostPort(reader.environment.Text("SMTP_INBOUND_HOST", "0.0.0.0"), reader.environment.Text("SMTP_INBOUND_PORT", "25")),
		Hostname:        reader.environment.Text("MAIL_HOSTNAME", reader.machineName()),
		MaxSize:         reader.number("SMTP_MAX_SIZE", protocol.Limits{}.DefaultMaxSize()),
		MaxConnections:  int(reader.number("SMTP_MAX_CONNECTIONS", int64(protocol.Limits{}.DefaultMaxConnections()))),
		CertificateFile: reader.environment.Optional(reader.certificateName()),
		KeyFile:         reader.environment.Optional(reader.keyName()),
		VerifyAuth:      !strings.EqualFold(reader.environment.Optional("MAIL_AUTH_VERIFY"), "off"),
		DirectoryUrl:    reader.required("MESAIL_DIRECTORY_URL"),
		MailInUrl:       reader.required("MESAIL_MAIL_IN_URL"),
		ServiceToken:    reader.environment.Optional("MESAIL_SERVICE_TOKEN"),
	}
	settings.TokenSettings = reader.tokenSettings(settings.ServiceToken)
	settings.RawStoreS3 = reader.s3RawStore()
	settings.RawStoreFolder = reader.rawStoreFolder(settings.RawStoreS3)
	reader.checkCertificatePair(settings)
	return settings, errors.Join(reader.problems...)
}

func (SettingsReader) machineName() string {
	name, err := os.Hostname()
	if err != nil {
		return "localhost"
	}
	return name
}

func (reader *SettingsReader) number(name string, fallback int64) int64 {
	value, err := strconv.ParseInt(reader.environment.Text(name, strconv.FormatInt(fallback, reader.decimal())), reader.decimal(), 64)
	if err != nil || value <= 0 {
		reader.problems = append(reader.problems, InvalidSetting{Name: name, Reason: "it must be a positive whole number"})
	}
	return value
}

func (reader *SettingsReader) seconds(name string, fallback time.Duration) time.Duration {
	return time.Duration(reader.number(name, int64(fallback/time.Second))) * time.Second
}

func (reader *SettingsReader) required(name string) string {
	value := reader.environment.Optional(name)
	if value == "" {
		reader.problems = append(reader.problems, MissingSetting{Name: name})
	}
	return value
}

func (reader *SettingsReader) tokenSettings(serviceToken string) adapters.TokenSettings {
	if serviceToken != "" {
		return adapters.TokenSettings{}
	}
	return reader.clientCredentials()
}

func (reader *SettingsReader) clientCredentials() adapters.TokenSettings {
	return adapters.TokenSettings{
		Url:        reader.required("MESAIL_SERVICE_TOKEN_URL"),
		ClientId:   reader.required("MESAIL_SERVICE_CLIENT_ID"),
		SecretFile: reader.required("MESAIL_SERVICE_CLIENT_SECRET_FILE"),
	}
}

func (reader *SettingsReader) s3RawStore() S3RawStoreSettings {
	names := S3RawStoreNames{}
	bucket := reader.environment.Optional(names.Bucket())
	if bucket == "" {
		return S3RawStoreSettings{}
	}
	return S3RawStoreSettings{
		Endpoint:    reader.environment.Optional(names.Endpoint()),
		Bucket:      bucket,
		Region:      reader.required(names.Region()),
		AccessKeyId: reader.required(names.AccessKeyId()),
		SecretFile:  reader.required(names.SecretFile()),
		Timeout:     reader.seconds(names.TimeoutSeconds(), RawStoreFactory{}.defaultUploadTimeout()),
	}
}

func (reader *SettingsReader) rawStoreFolder(s3 S3RawStoreSettings) string {
	if s3.Bucket != "" {
		return ""
	}
	return reader.required("MESAIL_RAW_STORE_DIR")
}

func (reader *SettingsReader) checkCertificatePair(settings ListenerSettings) {
	if (settings.CertificateFile == "") != (settings.KeyFile == "") {
		reader.problems = append(reader.problems, InvalidSetting{Name: reader.certificateName(), Reason: reader.certificateName() + " and " + reader.keyName() + " must be set together"})
	}
}

func (SettingsReader) certificateName() string {
	return "MAIL_TLS_CERT_FILE"
}

func (SettingsReader) decimal() int {
	return 10
}

func (SettingsReader) keyName() string {
	return "MAIL_TLS_KEY_FILE"
}
