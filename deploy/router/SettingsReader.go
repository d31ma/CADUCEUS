package main

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type SettingsReader struct {
	Lookup func(string) (string, bool)
}

func (reader SettingsReader) Read() (Settings, error) {
	upstreams, err := reader.pairs("MESAIL_ROUTER_UPSTREAMS")
	if err != nil {
		return Settings{}, err
	}
	staticFolders, err := reader.pairs("MESAIL_ROUTER_STATIC_FOLDERS")
	if err != nil {
		return Settings{}, err
	}
	rateLimit, err := reader.count("MESAIL_ROUTER_RATE_LIMIT", "10")
	if err != nil {
		return Settings{}, err
	}
	settings, err := reader.forMode(len(upstreams) > 0)
	settings.Upstreams = upstreams
	settings.StaticFolders = staticFolders
	settings.SingleWriters = reader.list("MESAIL_ROUTER_SINGLE_WRITERS", "")
	settings.Limited = reader.list("MESAIL_ROUTER_LIMITED_ROUTES", "POST /api/v1/identity/session,POST /api/v1/identity/session/refresh")
	settings.RateLimit = rateLimit
	return settings, err
}

func (reader SettingsReader) list(name string, fallback string) []string {
	return strings.FieldsFunc(reader.optional(name, fallback), reader.isComma)
}

func (SettingsReader) isComma(letter rune) bool {
	return letter == ','
}

func (reader SettingsReader) count(name string, fallback string) (int, error) {
	count, err := strconv.Atoi(reader.optional(name, fallback))
	if err != nil || count < 1 {
		return 0, InvalidSetting{Name: name, Reason: "use a whole number above 0"}
	}
	return count, nil
}

func (reader SettingsReader) forMode(native bool) (Settings, error) {
	if native {
		return reader.native()
	}
	return reader.lambda()
}

func (reader SettingsReader) native() (Settings, error) {
	timeout, err := reader.seconds("MESAIL_ROUTER_TIMEOUT_SECONDS", "70")
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		Listen:      reader.optional("MESAIL_ROUTER_LISTEN", ":8080"),
		KeycloakUrl: strings.TrimSuffix(reader.optional("MESAIL_ROUTER_KEYCLOAK_URL", ""), "/"),
		Timeout:     timeout,
		BodyLimit:   reader.bodyLimit(),
	}, nil
}

func (reader SettingsReader) lambda() (Settings, error) {
	services, err := reader.services()
	if err != nil {
		return Settings{}, err
	}
	lambdaEndpoint, err := reader.required("MESAIL_ROUTER_LAMBDA_ENDPOINT")
	if err != nil {
		return Settings{}, err
	}
	credentials, err := reader.credentials()
	if err != nil {
		return Settings{}, err
	}
	timeout, err := reader.seconds("MESAIL_ROUTER_TIMEOUT_SECONDS", "70")
	if err != nil {
		return Settings{}, err
	}
	return Settings{
		Listen:         reader.optional("MESAIL_ROUTER_LISTEN", ":8080"),
		Services:       services,
		FunctionPrefix: reader.optional("MESAIL_ROUTER_FUNCTION_PREFIX", "mesail-"),
		LambdaEndpoint: strings.TrimSuffix(lambdaEndpoint, "/"),
		S3Endpoint:     strings.TrimSuffix(reader.optional("MESAIL_ROUTER_S3_ENDPOINT", lambdaEndpoint), "/"),
		StaticBucket:   reader.optional("MESAIL_ROUTER_STATIC_BUCKET", "mesail-static"),
		KeycloakUrl:    strings.TrimSuffix(reader.optional("MESAIL_ROUTER_KEYCLOAK_URL", ""), "/"),
		Credentials:    credentials,
		Timeout:        timeout,
		BodyLimit:      reader.bodyLimit(),
	}, nil
}

func (reader SettingsReader) pairs(name string) (map[string]string, error) {
	pairs := map[string]string{}
	for _, entry := range strings.Split(reader.optional(name, ""), ",") {
		reader.addPair(pairs, entry)
	}
	return pairs, reader.checkPairs(name, pairs)
}

func (SettingsReader) addPair(pairs map[string]string, entry string) {
	key, value, found := strings.Cut(strings.TrimSpace(entry), "=")
	if found {
		pairs[key] = value
	}
}

func (reader SettingsReader) checkPairs(name string, pairs map[string]string) error {
	return reader.checkNamesOf(name, slices.Collect(maps.Keys(pairs)))
}

func (reader SettingsReader) credentials() (Credentials, error) {
	accessKey, err := reader.required("AWS_ACCESS_KEY_ID")
	if err != nil {
		return Credentials{}, err
	}
	secretKey, err := reader.required("AWS_SECRET_ACCESS_KEY")
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{
		AccessKey:    accessKey,
		SecretKey:    secretKey,
		SessionToken: reader.optional("AWS_SESSION_TOKEN", ""),
		Region:       reader.optional("AWS_REGION", "us-east-1"),
	}, nil
}

func (reader SettingsReader) services() ([]string, error) {
	list, err := reader.required("MESAIL_ROUTER_SERVICES")
	if err != nil {
		return nil, err
	}
	names := strings.Split(list, ",")
	return names, reader.checkNamesOf("MESAIL_ROUTER_SERVICES", names)
}

func (reader SettingsReader) checkNamesOf(setting string, names []string) error {
	index := slices.IndexFunc(names, reader.badName)
	if index >= 0 {
		return InvalidSetting{Name: setting, Reason: "bad name " + strconv.Quote(names[index])}
	}
	return nil
}

func (SettingsReader) badName(name string) bool {
	return !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(name)
}

func (reader SettingsReader) required(name string) (string, error) {
	value, found := reader.Lookup(name)
	if !found || value == "" {
		return "", MissingSetting{Name: name}
	}
	return value, nil
}

func (reader SettingsReader) optional(name string, fallback string) string {
	value, found := reader.Lookup(name)
	if !found || value == "" {
		return fallback
	}
	return value
}

func (reader SettingsReader) seconds(name string, fallback string) (time.Duration, error) {
	count, err := reader.count(name, fallback)
	return time.Duration(count) * time.Second, err
}

func (SettingsReader) bodyLimit() int64 {
	return 6 * 1024 * 1024
}
