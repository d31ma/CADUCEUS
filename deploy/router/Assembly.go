package main

import (
	"net/http"
	"net/url"
	"time"
)

type Assembly struct {
}

func (assembly Assembly) Handler(settings Settings) http.Handler {
	client := &http.Client{Timeout: settings.Timeout}
	return AccessLog{Next: Router{
		Ready:    ReadyHandler{},
		Api:      assembly.api(settings, client),
		Keycloak: assembly.keycloak(settings.KeycloakUrl),
		Static:   assembly.static(settings, client),
	}}
}

func (Assembly) api(settings Settings, client *http.Client) http.Handler {
	return ApiHandler{
		Routes:     Routes{}.Of(settings.Services, settings.FunctionPrefix),
		Bodies:     BodyReader{Limit: settings.BodyLimit},
		Events:     EventBuilder{Now: time.Now},
		Invoker:    LambdaInvoker{Endpoint: settings.LambdaEndpoint, Client: client, Signer: Signer{Credentials: settings.Credentials, Service: "lambda"}, Now: time.Now},
		Translator: ResultTranslator{},
	}
}

func (Assembly) static(settings Settings, client *http.Client) http.Handler {
	storage := S3Client{Endpoint: settings.S3Endpoint, Bucket: settings.StaticBucket, Client: client, Signer: Signer{Credentials: settings.Credentials, Service: "s3"}, Now: time.Now}
	return StaticHandler{Storage: storage, Paths: StaticPaths{}, Security: SecurityHeaders{}}
}

func (Assembly) keycloak(address string) http.Handler {
	target, err := url.Parse(address)
	if address == "" || err != nil {
		return NotFoundHandler{}
	}
	return KeycloakProxy{Target: target}.Handler()
}
