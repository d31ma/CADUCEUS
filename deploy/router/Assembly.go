package main

import (
	"net/http"
	"net/url"
	"time"
)

type Assembly struct {
}

func (assembly Assembly) Handler(settings Settings) (http.Handler, error) {
	client := &http.Client{Timeout: settings.Timeout}
	api, err := assembly.api(settings, client)
	if err != nil {
		return nil, err
	}
	return AccessLog{Next: Router{
		Ready:    ReadyHandler{},
		Api:      RateLimitHandler{Limited: settings.Limited, Limiter: RateLimiter{}.New(settings.RateLimit, assembly.rateWindow(), time.Now), Next: api},
		Keycloak: assembly.keycloak(settings.KeycloakUrl),
		Static:   StaticHandler{Storage: assembly.storage(settings, client), Paths: StaticPaths{}, Security: SecurityHeaders{}},
	}}, nil
}

func (assembly Assembly) api(settings Settings, client *http.Client) (http.Handler, error) {
	if len(settings.Upstreams) > 0 {
		return UpstreamHandler{}.Of(settings.Upstreams)
	}
	return assembly.lambda(settings, client), nil
}

func (Assembly) lambda(settings Settings, client *http.Client) http.Handler {
	return ApiHandler{
		Routes:     Routes{}.Of(settings.Services, settings.FunctionPrefix),
		Bodies:     BodyReader{Limit: settings.BodyLimit},
		Events:     EventBuilder{Now: time.Now},
		Invoker:    LambdaInvoker{Endpoint: settings.LambdaEndpoint, Client: client, Signer: Signer{Credentials: settings.Credentials, Service: "lambda"}, Now: time.Now, Limits: ConcurrencyLimits{}.Of(settings.Services, settings.SingleWriters, settings.FunctionPrefix, 1000)},
		Translator: ResultTranslator{},
	}
}

func (Assembly) storage(settings Settings, client *http.Client) ObjectStore {
	if len(settings.StaticFolders) > 0 {
		return FolderStore{Folders: settings.StaticFolders}
	}
	return S3Client{Endpoint: settings.S3Endpoint, Bucket: settings.StaticBucket, Client: client, Signer: Signer{Credentials: settings.Credentials, Service: "s3"}, Now: time.Now}
}

func (Assembly) rateWindow() time.Duration {
	return time.Minute
}

func (Assembly) keycloak(address string) http.Handler {
	target, err := url.Parse(address)
	if address == "" || err != nil {
		return NotFoundHandler{}
	}
	return KeycloakProxy{Target: target}.Handler()
}
