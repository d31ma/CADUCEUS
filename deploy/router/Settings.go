package main

import "time"

type Settings struct {
	Listen         string
	Services       []string
	FunctionPrefix string
	LambdaEndpoint string
	S3Endpoint     string
	StaticBucket   string
	KeycloakUrl    string
	Credentials    Credentials
	Timeout        time.Duration
	BodyLimit      int64
	Upstreams      map[string]string
	StaticFolders  map[string]string
	SingleWriters  []string
	Limited        []string
	RateLimit      int
	Passthrough    map[string]string
	Blocked        []string
	Blobs          BlobSettings
}
