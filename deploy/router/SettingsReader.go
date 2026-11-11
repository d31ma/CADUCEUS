package main

import (
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
	return names, reader.checkNames(names)
}

func (reader SettingsReader) checkNames(names []string) error {
	index := slices.IndexFunc(names, reader.badName)
	if index >= 0 {
		return InvalidSetting{Name: "MESAIL_ROUTER_SERVICES", Reason: "bad service name " + strconv.Quote(names[index])}
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
	count, err := strconv.Atoi(reader.optional(name, fallback))
	if err != nil || count < 1 {
		return 0, InvalidSetting{Name: name, Reason: "use a whole number of seconds"}
	}
	return time.Duration(count) * time.Second, nil
}

func (SettingsReader) bodyLimit() int64 {
	return 6 * 1024 * 1024
}
